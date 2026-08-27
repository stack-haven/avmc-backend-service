package data

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/evie/service/v1"
	"backend-service/app/evie/service/internal/biz"
	"backend-service/app/evie/service/internal/data/ent/gen"
	"backend-service/app/evie/service/internal/data/ent/gen/enhancementpolicy"
	"backend-service/app/evie/service/internal/data/ent/gen/enhancementprofile"
	pinyinpkg "backend-service/pkg/pinyin"
	"backend-service/pkg/aip/listing"
)

type enhancementPolicyRepo struct{ BaseRepo }

// NewEnhancementPolicyRepo 创建增强策略仓库。
func NewEnhancementPolicyRepo(data *Data, logger log.Logger) biz.EnhancementPolicyRepo {
	return &enhancementPolicyRepo{BaseRepo: NewBaseRepo(data, logger)}
}

func enhancementPolicyProto(row *gen.EnhancementPolicy) *pb.EnhancementPolicy {
	status := int32(0)
	if row.Status != nil {
		status = *row.Status
	}
	_ = status
	return &pb.EnhancementPolicy{
		Id:                     row.ID,
		Name:                   row.Name,
		Mode:                   row.Mode,
		TextCleaning:           row.TextCleaning,
		FillerRemoval:          row.FillerRemoval,
		AliasResolution:        row.AliasResolution,
		DeterministicReplacement: row.DeterministicReplacement,
		PinyinCorrection:       row.PinyinCorrection,
		FuzzyMatching:          row.FuzzyMatching,
		ContextCorrection:      row.ContextCorrection,
		Description:            row.Description,
		CreatedAt:              row.CreatedAt.Format(time.DateTime),
		UpdatedAt:              row.UpdatedAt.Format(time.DateTime),
	}
}

func enhancementProfileProto(row *gen.EnhancementProfile) *pb.EnhancementProfile {
	return &pb.EnhancementProfile{
		Id:          row.ID,
		PolicyId:    row.PolicyID,
		Name:        row.Name,
		Description: row.Description,
		CreatedAt:   row.CreatedAt.Format(time.DateTime),
		UpdatedAt:   row.UpdatedAt.Format(time.DateTime),
	}
}

// ListPolicies 分页查询策略。
func (r *enhancementPolicyRepo) ListPolicies(ctx context.Context, req *pb.ListPoliciesRequest) ([]*pb.EnhancementPolicy, int32, error) {
	query := r.Data.DB(ctx).EnhancementPolicy.Query()
	if req.GetKeyword() != "" {
		query.Where(enhancementpolicy.NameContains(req.GetKeyword()))
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	size := listing.NormalizePageSize(req.GetPageSize())
	offset := listing.PageOffset(req.GetPageToken())
	rows, err := query.Order(gen.Desc(enhancementpolicy.FieldID)).Offset(offset).Limit(size).All(ctx)
	if err != nil {
		return nil, 0, err
	}
	result := make([]*pb.EnhancementPolicy, 0, len(rows))
	for _, row := range rows {
		result = append(result, enhancementPolicyProto(row))
	}
	return result, int32(total), nil
}

// GetPolicy 查询策略详情。
func (r *enhancementPolicyRepo) GetPolicy(ctx context.Context, id uint32) (*pb.EnhancementPolicy, error) {
	row, err := r.Data.DB(ctx).EnhancementPolicy.Query().Where(enhancementpolicy.IDEQ(id)).Only(ctx)
	if gen.IsNotFound(err) {
		return nil, errors.NotFound("ENHANCEMENT_POLICY_NOT_FOUND", "增强策略不存在")
	}
	if err != nil {
		return nil, err
	}
	return enhancementPolicyProto(row), nil
}

// CreatePolicy 创建策略。
func (r *enhancementPolicyRepo) CreatePolicy(ctx context.Context, p *pb.EnhancementPolicy) (*pb.EnhancementPolicy, error) {
	row, err := r.Data.DB(ctx).EnhancementPolicy.Create().
		SetName(p.GetName()).
		SetMode(p.GetMode()).
		SetTextCleaning(p.GetTextCleaning()).
		SetFillerRemoval(p.GetFillerRemoval()).
		SetAliasResolution(p.GetAliasResolution()).
		SetDeterministicReplacement(p.GetDeterministicReplacement()).
		SetPinyinCorrection(p.GetPinyinCorrection()).
		SetFuzzyMatching(p.GetFuzzyMatching()).
		SetContextCorrection(p.GetContextCorrection()).
		SetDescription(p.GetDescription()).
		Save(ctx)
	if gen.IsConstraintError(err) {
		return nil, errors.Conflict("ENHANCEMENT_POLICY_EXISTS", "策略名称已存在")
	}
	if err != nil {
		return nil, err
	}
	return r.GetPolicy(ctx, row.ID)
}

// UpdatePolicy 更新策略。
func (r *enhancementPolicyRepo) UpdatePolicy(ctx context.Context, p *pb.EnhancementPolicy) (*pb.EnhancementPolicy, error) {
	update := r.Data.DB(ctx).EnhancementPolicy.UpdateOneID(p.GetId())
	if p.GetName() != "" {
		update.SetName(p.GetName())
	}
	if p.GetMode() != "" {
		update.SetMode(p.GetMode())
	}
	if p.GetDescription() != "" {
		update.SetDescription(p.GetDescription())
	}
	update.SetTextCleaning(p.GetTextCleaning()).
		SetFillerRemoval(p.GetFillerRemoval()).
		SetAliasResolution(p.GetAliasResolution()).
		SetDeterministicReplacement(p.GetDeterministicReplacement()).
		SetPinyinCorrection(p.GetPinyinCorrection()).
		SetFuzzyMatching(p.GetFuzzyMatching()).
		SetContextCorrection(p.GetContextCorrection())
	if _, err := update.Save(ctx); gen.IsNotFound(err) {
		return nil, errors.NotFound("ENHANCEMENT_POLICY_NOT_FOUND", "增强策略不存在")
	} else if err != nil {
		return nil, err
	}
	return r.GetPolicy(ctx, p.GetId())
}

// DeletePolicy 软删除策略。
func (r *enhancementPolicyRepo) DeletePolicy(ctx context.Context, id uint32) error {
	now := time.Now()
	if err := r.Data.DB(ctx).EnhancementPolicy.UpdateOneID(id).SetDeletedAt(now).Exec(ctx); gen.IsNotFound(err) {
		return errors.NotFound("ENHANCEMENT_POLICY_NOT_FOUND", "增强策略不存在")
	} else if err != nil {
		return err
	}
	return nil
}

// ListProfiles 分页查询场景。
func (r *enhancementPolicyRepo) ListProfiles(ctx context.Context, req *pb.ListProfilesRequest) ([]*pb.EnhancementProfile, int32, error) {
	query := r.Data.DB(ctx).EnhancementProfile.Query()
	if req.GetKeyword() != "" {
		query.Where(enhancementprofile.NameContains(req.GetKeyword()))
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	size := listing.NormalizePageSize(req.GetPageSize())
	offset := listing.PageOffset(req.GetPageToken())
	rows, err := query.Order(gen.Desc(enhancementprofile.FieldID)).Offset(offset).Limit(size).All(ctx)
	if err != nil {
		return nil, 0, err
	}
	result := make([]*pb.EnhancementProfile, 0, len(rows))
	for _, row := range rows {
		result = append(result, enhancementProfileProto(row))
	}
	return result, int32(total), nil
}

// GetProfile 查询场景详情。
func (r *enhancementPolicyRepo) GetProfile(ctx context.Context, id uint32) (*pb.EnhancementProfile, error) {
	row, err := r.Data.DB(ctx).EnhancementProfile.Query().Where(enhancementprofile.IDEQ(id)).Only(ctx)
	if gen.IsNotFound(err) {
		return nil, errors.NotFound("ENHANCEMENT_PROFILE_NOT_FOUND", "增强场景不存在")
	}
	if err != nil {
		return nil, err
	}
	return enhancementProfileProto(row), nil
}

// CreateProfile 创建场景。
func (r *enhancementPolicyRepo) CreateProfile(ctx context.Context, p *pb.EnhancementProfile) (*pb.EnhancementProfile, error) {
	row, err := r.Data.DB(ctx).EnhancementProfile.Create().
		SetPolicyID(p.GetPolicyId()).
		SetName(p.GetName()).
		SetDescription(p.GetDescription()).
		Save(ctx)
	if gen.IsConstraintError(err) {
		return nil, errors.Conflict("ENHANCEMENT_PROFILE_EXISTS", "场景名称已存在")
	}
	if err != nil {
		return nil, err
	}
	return r.GetProfile(ctx, row.ID)
}

// UpdateProfile 更新场景。
func (r *enhancementPolicyRepo) UpdateProfile(ctx context.Context, p *pb.EnhancementProfile) (*pb.EnhancementProfile, error) {
	update := r.Data.DB(ctx).EnhancementProfile.UpdateOneID(p.GetId())
	if p.GetPolicyId() != 0 {
		update.SetPolicyID(p.GetPolicyId())
	}
	if p.GetName() != "" {
		update.SetName(p.GetName())
	}
	if p.GetDescription() != "" {
		update.SetDescription(p.GetDescription())
	}
	if _, err := update.Save(ctx); gen.IsNotFound(err) {
		return nil, errors.NotFound("ENHANCEMENT_PROFILE_NOT_FOUND", "增强场景不存在")
	} else if err != nil {
		return nil, err
	}
	return r.GetProfile(ctx, p.GetId())
}

// DeleteProfile 软删除场景。
func (r *enhancementPolicyRepo) DeleteProfile(ctx context.Context, id uint32) error {
	now := time.Now()
	if err := r.Data.DB(ctx).EnhancementProfile.UpdateOneID(id).SetDeletedAt(now).Exec(ctx); gen.IsNotFound(err) {
		return errors.NotFound("ENHANCEMENT_PROFILE_NOT_FOUND", "增强场景不存在")
	} else if err != nil {
		return err
	}
	return nil
}

// GeneratePinyin 生成拼音。委托 pkg/pinyin 公共包，本接口为无状态工具，
// 不限租户（任何已登录用户可调用，用于前端表单辅助）。
func (r *enhancementPolicyRepo) GeneratePinyin(ctx context.Context, text string, includeInitials bool) (*pb.GeneratePinyinResponse, error) {
	_ = ctx // 未使用：拼音生成是函数式操作，不依赖上下文状态
	result, err := pinyinpkg.Convert(text, includeInitials)
	if err != nil {
		return nil, err
	}
	return &pb.GeneratePinyinResponse{
		Pinyin:         result.Pinyin,
		PinyinInitial:  result.PinyinInitial,
		NormalizedText: result.NormalizedText,
	}, nil
}
