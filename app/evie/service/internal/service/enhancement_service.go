package service

import (
	"context"

	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/evie/service/v1"
	"backend-service/app/evie/service/internal/biz"
)

// EnhancementServiceService 文本增强策略与场景 + 增强记录服务。
type EnhancementServiceService struct {
	pb.UnimplementedEnhancementServiceServer
	uc   *biz.EnhancementUsecase
	epuc *biz.EnhancementPolicyUsecase
	eluc *biz.EnhancementLogUsecase
	log  *log.Helper
}

// NewEnhancementServiceService 创建增强服务实例。
func NewEnhancementServiceService(uc *biz.EnhancementUsecase, epuc *biz.EnhancementPolicyUsecase, eluc *biz.EnhancementLogUsecase, logger log.Logger) *EnhancementServiceService {
	return &EnhancementServiceService{uc: uc, epuc: epuc, eluc: eluc, log: log.NewHelper(logger)}
}

// --- Policy ---

func (s *EnhancementServiceService) ListPolicies(ctx context.Context, req *pb.ListPoliciesRequest) (*pb.ListPoliciesResponse, error) {
	policies, total, err := s.epuc.ListPolicies(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pb.ListPoliciesResponse{Policies: policies, Total: total}, nil
}

func (s *EnhancementServiceService) GetPolicy(ctx context.Context, req *pb.GetPolicyRequest) (*pb.EnhancementPolicy, error) {
	return s.epuc.GetPolicy(ctx, req.GetId())
}

func (s *EnhancementServiceService) CreatePolicy(ctx context.Context, req *pb.CreatePolicyRequest) (*pb.EnhancementPolicy, error) {
	return s.epuc.CreatePolicy(ctx, &pb.EnhancementPolicy{
		Name:                     req.GetName(),
		Mode:                     req.GetMode(),
		TextCleaning:             req.GetTextCleaning(),
		FillerRemoval:            req.GetFillerRemoval(),
		AliasResolution:          req.GetAliasResolution(),
		DeterministicReplacement: req.GetDeterministicReplacement(),
		PinyinCorrection:         req.GetPinyinCorrection(),
		FuzzyMatching:            req.GetFuzzyMatching(),
		ContextCorrection:        req.GetContextCorrection(),
		Description:              req.GetDescription(),
	})
}

func (s *EnhancementServiceService) UpdatePolicy(ctx context.Context, req *pb.UpdatePolicyRequest) (*pb.EnhancementPolicy, error) {
	return s.epuc.UpdatePolicy(ctx, &pb.EnhancementPolicy{
		Id:                       req.GetId(),
		Name:                     req.GetName(),
		Mode:                     req.GetMode(),
		TextCleaning:             req.GetTextCleaning(),
		FillerRemoval:            req.GetFillerRemoval(),
		AliasResolution:          req.GetAliasResolution(),
		DeterministicReplacement: req.GetDeterministicReplacement(),
		PinyinCorrection:         req.GetPinyinCorrection(),
		FuzzyMatching:            req.GetFuzzyMatching(),
		ContextCorrection:        req.GetContextCorrection(),
		Description:              req.GetDescription(),
	})
}

func (s *EnhancementServiceService) DeletePolicy(ctx context.Context, req *pb.DeletePolicyRequest) (*pb.DeletePolicyResponse, error) {
	if err := s.epuc.DeletePolicy(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &pb.DeletePolicyResponse{}, nil
}

// --- Profile ---

func (s *EnhancementServiceService) ListProfiles(ctx context.Context, req *pb.ListProfilesRequest) (*pb.ListProfilesResponse, error) {
	profiles, total, err := s.epuc.ListProfiles(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pb.ListProfilesResponse{Profiles: profiles, Total: total}, nil
}

func (s *EnhancementServiceService) GetProfile(ctx context.Context, req *pb.GetProfileRequest) (*pb.EnhancementProfile, error) {
	return s.epuc.GetProfile(ctx, req.GetId())
}

func (s *EnhancementServiceService) CreateProfile(ctx context.Context, req *pb.CreateProfileRequest) (*pb.EnhancementProfile, error) {
	return s.epuc.CreateProfile(ctx, &pb.EnhancementProfile{
		PolicyId:    req.GetPolicyId(),
		Name:        req.GetName(),
		Description: req.GetDescription(),
	})
}

func (s *EnhancementServiceService) UpdateProfile(ctx context.Context, req *pb.UpdateProfileRequest) (*pb.EnhancementProfile, error) {
	return s.epuc.UpdateProfile(ctx, &pb.EnhancementProfile{
		Id:          req.GetId(),
		PolicyId:    req.GetPolicyId(),
		Name:        req.GetName(),
		Description: req.GetDescription(),
	})
}

func (s *EnhancementServiceService) DeleteProfile(ctx context.Context, req *pb.DeleteProfileRequest) (*pb.DeleteProfileResponse, error) {
	if err := s.epuc.DeleteProfile(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &pb.DeleteProfileResponse{}, nil
}

// --- 增强记录 ---

func (s *EnhancementServiceService) ListLogs(ctx context.Context, req *pb.ListEnhancementLogsRequest) (*pb.ListEnhancementLogsResponse, error) {
	logs, total, err := s.eluc.List(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pb.ListEnhancementLogsResponse{Logs: logs, Total: total}, nil
}

func (s *EnhancementServiceService) GetLog(ctx context.Context, req *pb.GetEnhancementLogRequest) (*pb.EnhancementLog, error) {
	return s.eluc.Get(ctx, req.GetId())
}

// GeneratePinyin 生成拼音（后端兑底，前端 pinyin-pro 失败时调用）。
// 本接口为无状态工具，不限租户——任何已登录用户可调用。
func (s *EnhancementServiceService) GeneratePinyin(ctx context.Context, req *pb.GeneratePinyinRequest) (*pb.GeneratePinyinResponse, error) {
	resp, err := s.epuc.GeneratePinyin(ctx, req.GetText(), req.GetIncludeInitials())
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// EnhanceText 纯文本增强（不经 ASR）：biz 层统一编排 8 层流水线、session_id 生成与落日志。
// profile_id 决定策略：0=租户默认；非 0=场景关联策略。
func (s *EnhancementServiceService) EnhanceText(ctx context.Context, req *pb.EnhanceTextRequest) (*pb.EnhanceTextResponse, error) {
	if req.GetText() == "" {
		return nil, kerrors.BadRequest("TEXT_REQUIRED", "文本不能为空")
	}
	result, err := s.uc.EnhanceText(ctx, req)
	if err != nil {
		return nil, err
	}
	return result.ToProto(), nil
}
