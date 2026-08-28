package service

import (
	"context"
	"encoding/json"

	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/evie/service/v1"
	"backend-service/app/evie/service/internal/biz"
	"backend-service/pkg/auth/authn"
)

// EnhancementServiceService 文本增强策略与场景 + 增强记录服务。
type EnhancementServiceService struct {
	pb.UnimplementedEnhancementServiceServer
	enhancer *biz.EnhancementEngine
	policyUc *biz.EnhancementPolicyUsecase
	logUc    *biz.EnhancementLogUsecase
	log      *log.Helper
}

// NewEnhancementServiceService 创建增强服务实例。
func NewEnhancementServiceService(enhancer *biz.EnhancementEngine, policyUc *biz.EnhancementPolicyUsecase, logUc *biz.EnhancementLogUsecase, logger log.Logger) *EnhancementServiceService {
	return &EnhancementServiceService{enhancer: enhancer, policyUc: policyUc, logUc: logUc, log: log.NewHelper(logger)}
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

// GeneratePinyin 生成拼音（后端兑底，前端 pinyin-pro 失败时调用）。
// 本接口为无状态工具，不限租户——任何已登录用户可调用。
func (s *EnhancementServiceService) GeneratePinyin(ctx context.Context, req *pb.GeneratePinyinRequest) (*pb.GeneratePinyinResponse, error) {
	resp, err := s.policyUc.GeneratePinyin(ctx, req.GetText(), req.GetIncludeInitials())
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// EnhanceText 纯文本增强（不经 ASR）：8 层流水线 + 落 EnhancementLog。
// profile_id 决定策略：0=租户默认；非 0=场景关联策略。
func (s *EnhancementServiceService) EnhanceText(ctx context.Context, req *pb.EnhanceTextRequest) (*pb.EnhanceTextResponse, error) {
	if req.GetText() == "" {
		return nil, kerrors.BadRequest("TEXT_REQUIRED", "文本不能为空")
	}
	tenantID := authn.GetAuthUserTenantID(ctx)
	policy, err := resolveEnhancePolicy(ctx, s.policyUc, req.GetProfileId())
	if err != nil {
		return nil, err
	}
	result, err := s.enhancer.EnhanceWithPolicy(ctx, tenantID, req.GetText(), policy)
	if err != nil {
		return nil, err
	}

	// 落增强日志
	changes := make([]*pb.EnhanceChange, 0, len(result.Changes))
	for _, ch := range result.Changes {
		changes = append(changes, &pb.EnhanceChange{
			From:       ch.OriginalText,
			To:         ch.ResultText,
			Type:       ch.Type,
			Confidence: float32(ch.Confidence),
		})
	}
	if s.logUc != nil {
		changesJSON, _ := json.Marshal(result.Changes)
		snapshotsJSON, _ := json.Marshal(result.StepSnapshots)
		s.logUc.Save(ctx, &biz.EnhancementLogData{
			SessionID:           req.GetSessionId(),
			RawText:             result.RawText,
			EnhancedText:        result.EnhancedText,
			ChangesJSON:         string(changesJSON),
			StepSnapshotsJSON:   string(snapshotsJSON),
			ProcessingTimeMs:    result.TotalTime.Milliseconds(),
			CleaningTimeMs:      stepMs(result.StepTimings, "cleaning"),
			FillerTimeMs:        stepMs(result.StepTimings, "filler"),
			VocabMatchTimeMs:    stepMs(result.StepTimings, "vocabulary_matching"),
			AliasTimeMs:         stepMs(result.StepTimings, "alias_resolution"),
			DeterministicTimeMs: stepMs(result.StepTimings, "deterministic_replacement"),
			PinyinTimeMs:        stepMs(result.StepTimings, "pinyin_correction"),
			FuzzyTimeMs:         stepMs(result.StepTimings, "fuzzy_matching"),
			ContextTimeMs:       stepMs(result.StepTimings, "context_correction"),
			Status:              1,
		})
	}

	// 找不到任何策略时返回原文（corrected==original），status 仍 SUCCESS（不算失败）
	enhanced := result.EnhancedText
	if enhanced == "" {
		enhanced = result.RawText
	}
	return &pb.EnhanceTextResponse{
		OriginalText:       result.RawText,
		EnhancedText:       enhanced,
		Changes:            changes,
		Status:             1,
		ProcessingTimeMs:   result.TotalTime.Milliseconds(),
		CleaningTimeMs:     stepMs(result.StepTimings, "cleaning"),
		FillerTimeMs:       stepMs(result.StepTimings, "filler"),
		VocabMatchTimeMs:   stepMs(result.StepTimings, "vocabulary_matching"),
		AliasTimeMs:        stepMs(result.StepTimings, "alias_resolution"),
		DeterministicTimeMs: stepMs(result.StepTimings, "deterministic_replacement"),
		PinyinTimeMs:       stepMs(result.StepTimings, "pinyin_correction"),
		FuzzyTimeMs:        stepMs(result.StepTimings, "fuzzy_matching"),
		ContextTimeMs:      stepMs(result.StepTimings, "context_correction"),
	}, nil
}
