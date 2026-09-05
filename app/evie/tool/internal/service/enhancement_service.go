// Package service · enhancement_service.go
// EnhancementService：文本增强 transport 层。
//
// 责任：
//   1. 接收 HTTP/gRPC 请求（*v1.EnhanceTextRequest）
//   2. 从 ctx 提取 AuthInfo（tenantID）
//   3. 调 usecase.EnhanceText(ctx, text, tenantID)
//   4. 把 usecase 结果转 proto（*v1.EnhanceTextResponse）
package service

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	v1 "backend-service/api/evie/tool/v1"
	"backend-service/app/evie/tool/internal/biz"
	"backend-service/app/evie/tool/internal/data"
)

// EnhancementService 文本增强服务。
type EnhancementService struct {
	v1.UnimplementedEnhancementServiceServer

	uc  *biz.EnhancementUsecase
	log *log.Helper
}

// NewEnhancementService 构造。
func NewEnhancementService(uc *biz.EnhancementUsecase, logger log.Logger) *EnhancementService {
	return &EnhancementService{
		uc:  uc,
		log: log.NewHelper(log.With(logger, "module", "service/enhancement")),
	}
}

// EnhanceText 实现 EnhancementServiceServer.EnhanceText。
func (s *EnhancementService) EnhanceText(ctx context.Context, req *v1.EnhanceTextRequest) (*v1.EnhanceTextResponse, error) {
	// 1. 从 ctx 拿 AuthInfo
	auth, ok := data.AuthInfoFromContext(ctx)
	if !ok {
		return nil, v1.ErrorTokenMissing("enhance text requires auth context")
	}
	tenantID := auth.TenantID

	// 2. 调 usecase
	res, err := s.uc.EnhanceText(ctx, req.GetText(), tenantID)
	if err != nil {
		s.log.Warnf("EnhanceText failed: %v", err)
		return nil, err
	}

	// 3. 转 proto
	return &v1.EnhanceTextResponse{
		OriginalText:        res.OriginalText,
		EnhancedText:        res.EnhancedText,
		Changes:             res.Changes,
		Status:              res.Status,
		ProcessingTimeMs:    res.ProcessingTimeMs,
		CleaningTimeMs:      res.CleaningTimeMs,
		FillerTimeMs:        res.FillerTimeMs,
		VocabMatchTimeMs:    res.VocabMatchTimeMs,
		AliasTimeMs:         res.AliasTimeMs,
		DeterministicTimeMs: res.DeterministicTimeMs,
		PinyinTimeMs:        res.PinyinTimeMs,
		FuzzyTimeMs:         res.FuzzyTimeMs,
		ContextTimeMs:       res.ContextTimeMs,
		ErrorMessage:        res.ErrorMessage,
	}, nil
}