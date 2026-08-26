package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/evie/service/v1"
	"backend-service/app/evie/service/internal/biz"
	"backend-service/pkg/auth/authn"
)

// CorrectionServiceService 文本增强引擎服务（对 ASR 原文执行清洗/别名/纠错）。
type CorrectionServiceService struct {
	pb.UnimplementedCorrectionServiceServer
	enhancer *biz.EnhancementEngine
	policyUc *biz.EnhancementPolicyUsecase
	logUc    *biz.EnhancementLogUsecase
	log      *log.Helper
}

// NewCorrectionServiceService 创建文本增强服务实例。
func NewCorrectionServiceService(enhancer *biz.EnhancementEngine, policyUc *biz.EnhancementPolicyUsecase, logUc *biz.EnhancementLogUsecase, logger log.Logger) *CorrectionServiceService {
	return &CorrectionServiceService{enhancer: enhancer, policyUc: policyUc, logUc: logUc, log: log.NewHelper(logger)}
}

// Correct 文本增强（清洗 → 口水词 → 词库匹配 → 别名/纠错 → 拼音/模糊/上下文）。
func (s *CorrectionServiceService) Correct(ctx context.Context, req *pb.CorrectRequest) (*pb.CorrectResponse, error) {
	tenantID := authn.GetAuthUserTenantID(ctx)
	policy, err := resolveEnhancePolicy(ctx, s.policyUc, req.GetProfileId())
	if err != nil {
		return nil, err
	}
	result, err := s.enhancer.EnhanceWithPolicy(ctx, tenantID, req.GetText(), policy)
	if err != nil {
		return nil, err
	}

	changes := make([]*pb.CorrectionChange, 0, len(result.Changes))
	needConfirm := false
	for _, ch := range result.Changes {
		changes = append(changes, &pb.CorrectionChange{
			From:       ch.OriginalText,
			To:         ch.ResultText,
			Type:       ch.Type,
			Confidence: float32(ch.Confidence),
		})
		if ch.Action == biz.ActionSuggest {
			needConfirm = true
		}
	}

	// 保存增强日志（分阶段耗时 + 变更）
	if s.logUc != nil && result.StepTimings != nil {
		changesJSON, _ := json.Marshal(result.Changes)
		s.logUc.Save(ctx, &biz.EnhancementLogData{
			RawText:             result.RawText,
			EnhancedText:        result.EnhancedText,
			ChangesJSON:         string(changesJSON),
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

	return &pb.CorrectResponse{
		OriginalText:  result.RawText,
		CorrectedText: result.EnhancedText,
		Changes:       changes,
		NeedConfirm:   needConfirm,
	}, nil
}

// stepMs 安全获取步骤耗时（毫秒）。
func stepMs(timings map[string]time.Duration, name string) int64 {
	if d, ok := timings[name]; ok {
		return d.Milliseconds()
	}
	return 0
}
