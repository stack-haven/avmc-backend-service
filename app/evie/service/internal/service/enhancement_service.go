package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/evie/service/v1"
	"backend-service/app/evie/service/internal/biz"
)

// EnhancementServiceService 文本增强策略与场景 + 增强记录服务。
type EnhancementServiceService struct {
	pb.UnimplementedEnhancementServiceServer
	policyUc *biz.EnhancementPolicyUsecase
	logUc    *biz.EnhancementLogUsecase
	log      *log.Helper
}

// NewEnhancementServiceService 创建增强服务实例。
func NewEnhancementServiceService(policyUc *biz.EnhancementPolicyUsecase, logUc *biz.EnhancementLogUsecase, logger log.Logger) *EnhancementServiceService {
	return &EnhancementServiceService{policyUc: policyUc, logUc: logUc, log: log.NewHelper(logger)}
}

// --- Policy ---

func (s *EnhancementServiceService) ListPolicies(ctx context.Context, req *pb.ListPoliciesRequest) (*pb.ListPoliciesResponse, error) {
	policies, total, err := s.policyUc.ListPolicies(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pb.ListPoliciesResponse{Policies: policies, Total: total}, nil
}

func (s *EnhancementServiceService) GetPolicy(ctx context.Context, req *pb.GetPolicyRequest) (*pb.EnhancementPolicy, error) {
	return s.policyUc.GetPolicy(ctx, req.GetId())
}

func (s *EnhancementServiceService) CreatePolicy(ctx context.Context, req *pb.CreatePolicyRequest) (*pb.EnhancementPolicy, error) {
	return s.policyUc.CreatePolicy(ctx, &pb.EnhancementPolicy{
		Name:                   req.GetName(),
		Mode:                   req.GetMode(),
		TextCleaning:           req.GetTextCleaning(),
		FillerRemoval:          req.GetFillerRemoval(),
		AliasResolution:        req.GetAliasResolution(),
		DeterministicReplacement: req.GetDeterministicReplacement(),
		PinyinCorrection:       req.GetPinyinCorrection(),
		FuzzyMatching:          req.GetFuzzyMatching(),
		ContextCorrection:      req.GetContextCorrection(),
		Description:            req.GetDescription(),
	})
}

func (s *EnhancementServiceService) UpdatePolicy(ctx context.Context, req *pb.UpdatePolicyRequest) (*pb.EnhancementPolicy, error) {
	return s.policyUc.UpdatePolicy(ctx, &pb.EnhancementPolicy{
		Id:                     req.GetId(),
		Name:                   req.GetName(),
		Mode:                   req.GetMode(),
		TextCleaning:           req.GetTextCleaning(),
		FillerRemoval:          req.GetFillerRemoval(),
		AliasResolution:        req.GetAliasResolution(),
		DeterministicReplacement: req.GetDeterministicReplacement(),
		PinyinCorrection:       req.GetPinyinCorrection(),
		FuzzyMatching:          req.GetFuzzyMatching(),
		ContextCorrection:      req.GetContextCorrection(),
		Description:            req.GetDescription(),
	})
}

func (s *EnhancementServiceService) DeletePolicy(ctx context.Context, req *pb.DeletePolicyRequest) (*pb.DeletePolicyResponse, error) {
	if err := s.policyUc.DeletePolicy(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &pb.DeletePolicyResponse{}, nil
}

// --- Profile ---

func (s *EnhancementServiceService) ListProfiles(ctx context.Context, req *pb.ListProfilesRequest) (*pb.ListProfilesResponse, error) {
	profiles, total, err := s.policyUc.ListProfiles(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pb.ListProfilesResponse{Profiles: profiles, Total: total}, nil
}

func (s *EnhancementServiceService) GetProfile(ctx context.Context, req *pb.GetProfileRequest) (*pb.EnhancementProfile, error) {
	return s.policyUc.GetProfile(ctx, req.GetId())
}

func (s *EnhancementServiceService) CreateProfile(ctx context.Context, req *pb.CreateProfileRequest) (*pb.EnhancementProfile, error) {
	return s.policyUc.CreateProfile(ctx, &pb.EnhancementProfile{
		PolicyId:    req.GetPolicyId(),
		Name:        req.GetName(),
		Description: req.GetDescription(),
	})
}

func (s *EnhancementServiceService) UpdateProfile(ctx context.Context, req *pb.UpdateProfileRequest) (*pb.EnhancementProfile, error) {
	return s.policyUc.UpdateProfile(ctx, &pb.EnhancementProfile{
		Id:          req.GetId(),
		PolicyId:    req.GetPolicyId(),
		Name:        req.GetName(),
		Description: req.GetDescription(),
	})
}

func (s *EnhancementServiceService) DeleteProfile(ctx context.Context, req *pb.DeleteProfileRequest) (*pb.DeleteProfileResponse, error) {
	if err := s.policyUc.DeleteProfile(ctx, req.GetId()); err != nil {
		return nil, err
	}
	return &pb.DeleteProfileResponse{}, nil
}

// --- 增强记录 ---

func (s *EnhancementServiceService) ListLogs(ctx context.Context, req *pb.ListEnhancementLogsRequest) (*pb.ListEnhancementLogsResponse, error) {
	logs, total, err := s.logUc.List(ctx, req)
	if err != nil {
		return nil, err
	}
	return &pb.ListEnhancementLogsResponse{Logs: logs, Total: total}, nil
}

func (s *EnhancementServiceService) GetLog(ctx context.Context, req *pb.GetEnhancementLogRequest) (*pb.EnhancementLog, error) {
	return s.logUc.Get(ctx, req.GetId())
}
