// Package biz · enhancement.go
// EnhancementUsecase：文本增强业务编排。
//
// 责任：
//   1. 拿 ctx 中的 AuthInfo（tenantID）
//   2. 用 VocabularyBuilder.Build(tenantID) 取不可变快照
//   3. 构造 EnhancementContext 喂给 textenhance.Pipeline
//   4. 读 ec 结果 + status + 错误
//   5. 转成业务 DTO（EnhanceTextResult）返回给 service 层
package biz

import (
	"context"
	"fmt"

	"backend-service/pkg/textenhance"

	v1 "backend-service/api/evie/tool/v1"
)

// EnhanceTextResult 业务结果（service 层转 proto）。
type EnhanceTextResult struct {
	OriginalText        string
	EnhancedText        string
	Changes             []*v1.EnhanceChange
	Status              int32
	ProcessingTimeMs    int64
	CleaningTimeMs      int64
	FillerTimeMs        int64
	VocabMatchTimeMs    int64
	AliasTimeMs         int64
	DeterministicTimeMs int64
	PinyinTimeMs        int64
	FuzzyTimeMs         int64
	ContextTimeMs       int64
	ErrorMessage        string
}

// EnhancementUsecase 文本增强用例。
type EnhancementUsecase struct {
	pipeline     *textenhance.Pipeline
	vocabBuilder *VocabularyBuilder
	policy       *textenhance.Policy
}

// NewEnhancementUsecase 构造。
func NewEnhancementUsecase(
	pipeline *textenhance.Pipeline,
	vocabBuilder *VocabularyBuilder,
	policy *textenhance.Policy,
) *EnhancementUsecase {
	return &EnhancementUsecase{
		pipeline:     pipeline,
		vocabBuilder: vocabBuilder,
		policy:       policy,
	}
}

// EnhanceText 增强一段文本（指定 tenantID）。
//
// 设计：tenantID 由 service 层从 ctx 提取后传入；biz 层不依赖 data 层
//（避免 data → biz → data 的 import cycle）。
func (uc *EnhancementUsecase) EnhanceText(ctx context.Context, rawText, tenantID string) (*EnhanceTextResult, error) {
	if rawText == "" {
		return nil, fmt.Errorf("biz: rawText is empty")
	}
	if tenantID == "" {
		return nil, fmt.Errorf("biz: tenantID is empty (auth missing?)")
	}

	// 1. 拿词库快照
	vocab := uc.vocabBuilder.Build(ctx, tenantID)

	// 2. 构造 ec + 跑 pipeline
	ec := textenhance.NewEnhancementContext(rawText, vocab, uc.policy)
	uc.pipeline.Run(ctx, ec)

	// 3. 读结果
	res := &EnhanceTextResult{
		OriginalText:        ec.GetRawText(),
		EnhancedText:        ec.GetText(),
		Changes:             convertChanges(ec.GetChanges()),
		Status:              ec.GetStatus(),
		ProcessingTimeMs:    ec.Elapsed().Milliseconds(),
		CleaningTimeMs:      lookupTiming(ec, "cleaning"),
		FillerTimeMs:        lookupTiming(ec, "filler"),
		VocabMatchTimeMs:    lookupTiming(ec, "vocab_matching"),
		AliasTimeMs:         lookupTiming(ec, "alias_resolution"),
		DeterministicTimeMs: lookupTiming(ec, "deterministic_replacement"),
		PinyinTimeMs:        lookupTiming(ec, "pinyin_correction"),
		FuzzyTimeMs:         lookupTiming(ec, "fuzzy_matching"),
		ContextTimeMs:       lookupTiming(ec, "context_correction"),
		ErrorMessage:        ec.JoinErrors(),
	}

	return res, nil
}

// EnhanceTextForTenant 兼容旧名（保留 API 兼容；同 EnhanceText）。
func (uc *EnhancementUsecase) EnhanceTextForTenant(ctx context.Context, rawText, tenantID string) (*EnhanceTextResult, error) {
	if rawText == "" {
		return nil, fmt.Errorf("biz: rawText is empty")
	}
	vocab := uc.vocabBuilder.Build(ctx, tenantID)
	ec := textenhance.NewEnhancementContext(rawText, vocab, uc.policy)
	uc.pipeline.Run(ctx, ec)
	return &EnhanceTextResult{
		OriginalText:        ec.GetRawText(),
		EnhancedText:        ec.GetText(),
		Changes:             convertChanges(ec.GetChanges()),
		Status:              ec.GetStatus(),
		ProcessingTimeMs:    ec.Elapsed().Milliseconds(),
		CleaningTimeMs:      lookupTiming(ec, "cleaning"),
		FillerTimeMs:        lookupTiming(ec, "filler"),
		VocabMatchTimeMs:    lookupTiming(ec, "vocab_matching"),
		AliasTimeMs:         lookupTiming(ec, "alias_resolution"),
		DeterministicTimeMs: lookupTiming(ec, "deterministic_replacement"),
		PinyinTimeMs:        lookupTiming(ec, "pinyin_correction"),
		FuzzyTimeMs:         lookupTiming(ec, "fuzzy_matching"),
		ContextTimeMs:       lookupTiming(ec, "context_correction"),
		ErrorMessage:        ec.JoinErrors(),
	}, nil
}

// convertChanges 把 processors.Change 转 v1.EnhanceChange。
func convertChanges(cs []textenhance.Change) []*v1.EnhanceChange {
	out := make([]*v1.EnhanceChange, 0, len(cs))
	for _, c := range cs {
		out = append(out, &v1.EnhanceChange{
			From:       c.From,
			To:         c.To,
			Action:     c.Action,
			Type:       c.Type,
			Source:     c.Source,
			Confidence: float32(c.Confidence),
			Locked:     c.Locked,
			Reason:     c.Reason,
		})
	}
	return out
}

// lookupTiming 从 ec.Timings 查指定 step 的耗时。
func lookupTiming(ec *textenhance.EnhancementContext, name string) int64 {
	t := ec.GetTimings()
	if d, ok := t[name]; ok {
		return d.Milliseconds()
	}
	return 0
}