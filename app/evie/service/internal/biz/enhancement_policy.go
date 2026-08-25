package biz

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/evie/service/v1"
)

// EnhancementPolicyRepo 增强策略与场景仓库接口。
type EnhancementPolicyRepo interface {
	ListPolicies(ctx context.Context, req *pb.ListPoliciesRequest) ([]*pb.EnhancementPolicy, int32, error)
	GetPolicy(ctx context.Context, id uint32) (*pb.EnhancementPolicy, error)
	CreatePolicy(ctx context.Context, policy *pb.EnhancementPolicy) (*pb.EnhancementPolicy, error)
	UpdatePolicy(ctx context.Context, policy *pb.EnhancementPolicy) (*pb.EnhancementPolicy, error)
	DeletePolicy(ctx context.Context, id uint32) error

	ListProfiles(ctx context.Context, req *pb.ListProfilesRequest) ([]*pb.EnhancementProfile, int32, error)
	GetProfile(ctx context.Context, id uint32) (*pb.EnhancementProfile, error)
	CreateProfile(ctx context.Context, profile *pb.EnhancementProfile) (*pb.EnhancementProfile, error)
	UpdateProfile(ctx context.Context, profile *pb.EnhancementProfile) (*pb.EnhancementProfile, error)
	DeleteProfile(ctx context.Context, id uint32) error
}

// EnhancementPolicyUsecase 增强策略与场景业务逻辑。
type EnhancementPolicyUsecase struct {
	repo EnhancementPolicyRepo
	log  *log.Helper
}

// NewEnhancementPolicyUsecase 创建增强策略 usecase。
func NewEnhancementPolicyUsecase(repo EnhancementPolicyRepo, logger log.Logger) *EnhancementPolicyUsecase {
	return &EnhancementPolicyUsecase{repo: repo, log: log.NewHelper(logger)}
}

// ListPolicies 分页查询策略。
func (uc *EnhancementPolicyUsecase) ListPolicies(ctx context.Context, req *pb.ListPoliciesRequest) ([]*pb.EnhancementPolicy, int32, error) {
	return uc.repo.ListPolicies(ctx, req)
}

// GetPolicy 查询策略详情。
func (uc *EnhancementPolicyUsecase) GetPolicy(ctx context.Context, id uint32) (*pb.EnhancementPolicy, error) {
	return uc.repo.GetPolicy(ctx, id)
}

// CreatePolicy 创建策略。
func (uc *EnhancementPolicyUsecase) CreatePolicy(ctx context.Context, policy *pb.EnhancementPolicy) (*pb.EnhancementPolicy, error) {
	if policy.GetName() == "" {
		return nil, pb.ErrorBadRequest("策略名称不能为空")
	}
	if policy.GetMode() == "" {
		policy.Mode = "STANDARD"
	}
	return uc.repo.CreatePolicy(ctx, policy)
}

// UpdatePolicy 更新策略。
func (uc *EnhancementPolicyUsecase) UpdatePolicy(ctx context.Context, policy *pb.EnhancementPolicy) (*pb.EnhancementPolicy, error) {
	if policy.GetId() == 0 {
		return nil, pb.ErrorBadRequest("策略ID不能为空")
	}
	return uc.repo.UpdatePolicy(ctx, policy)
}

// DeletePolicy 删除策略。
func (uc *EnhancementPolicyUsecase) DeletePolicy(ctx context.Context, id uint32) error {
	if id == 0 {
		return pb.ErrorBadRequest("策略ID不能为空")
	}
	return uc.repo.DeletePolicy(ctx, id)
}

// ListProfiles 分页查询场景。
func (uc *EnhancementPolicyUsecase) ListProfiles(ctx context.Context, req *pb.ListProfilesRequest) ([]*pb.EnhancementProfile, int32, error) {
	return uc.repo.ListProfiles(ctx, req)
}

// GetProfile 查询场景详情。
func (uc *EnhancementPolicyUsecase) GetProfile(ctx context.Context, id uint32) (*pb.EnhancementProfile, error) {
	return uc.repo.GetProfile(ctx, id)
}

// CreateProfile 创建场景。
func (uc *EnhancementPolicyUsecase) CreateProfile(ctx context.Context, profile *pb.EnhancementProfile) (*pb.EnhancementProfile, error) {
	if profile.GetPolicyId() == 0 {
		return nil, pb.ErrorBadRequest("场景必须绑定策略")
	}
	if profile.GetName() == "" {
		return nil, pb.ErrorBadRequest("场景名称不能为空")
	}
	return uc.repo.CreateProfile(ctx, profile)
}

// UpdateProfile 更新场景。
func (uc *EnhancementPolicyUsecase) UpdateProfile(ctx context.Context, profile *pb.EnhancementProfile) (*pb.EnhancementProfile, error) {
	if profile.GetId() == 0 {
		return nil, pb.ErrorBadRequest("场景ID不能为空")
	}
	return uc.repo.UpdateProfile(ctx, profile)
}

// DeleteProfile 删除场景。
func (uc *EnhancementPolicyUsecase) DeleteProfile(ctx context.Context, id uint32) error {
	if id == 0 {
		return pb.ErrorBadRequest("场景ID不能为空")
	}
	return uc.repo.DeleteProfile(ctx, id)
}
