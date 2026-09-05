// Package biz · enhancement.go
// EnhancementUsecase：基于 lexnorm 引擎的文本增强业务编排。
//
// M9.6 改造：从 pkg/textenhance 切换到 pkg/lexnorm。
//
// 责任：
//   1. 拿 ctx 中的 AuthInfo（tenantID）
//   2. 调 lexnorm.Engine.Normalize（内部 lazy 加载 tenant 词库）
//   3. lexnorm.Change → v1.EnhanceChange 转换（保持 HTTP API 兼容）
//   4. 读 result.Changes + Steps + Errors
//   5. 返回 EnhanceTextResult
package biz

import (
	"context"
	"fmt"

	"github.com/stack-haven/lexnorm"

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

// EnhancementUsecase 文本增强用例（lexnorm 版本）。
type EnhancementUsecase struct {
	engine *lexnorm.Engine
}

// NewEnhancementUsecase 构造（注入 lexnorm.Engine）。
//
// 注意：旧版 NewEnhancementUsecase(pipeline, builder, policy) 已废弃。
// wire 直接注入 lexnorm.Engine。
func NewEnhancementUsecase(engine *lexnorm.Engine) *EnhancementUsecase {
	return &EnhancementUsecase{engine: engine}
}

// EnhanceText 增强一段文本（指定 tenantID）。
//
// tenantID 来自 ctx 中的 AuthInfo（service 层提取后传入）。
func (uc *EnhancementUsecase) EnhanceText(ctx context.Context, rawText, tenantID string) (*EnhanceTextResult, error) {
	if rawText == "" {
		return nil, fmt.Errorf("biz: rawText is empty")
	}
	if tenantID == "" {
		return nil, fmt.Errorf("biz: tenantID is empty (auth missing?)")
	}

	// 1. 跑 lexnorm 引擎
	res, err := uc.engine.Normalize(ctx, rawText, lexnorm.WithProfileID(lexnorm.ProfileID(tenantID)))
	if err != nil {
		return nil, fmt.Errorf("biz: lexnorm.Normalize failed: %w", err)
	}

	// 2. 转 Change
	changes := convertChanges(res.Changes)

	// 3. 读 Steps 拿每步耗时（按 Processor.Name 索引）
	timings := make(map[string]int64, len(res.Steps))
	for _, st := range res.Steps {
		timings[st.Processor] = st.Duration.Milliseconds()
	}

	return &EnhanceTextResult{
		OriginalText:        res.Original,
		EnhancedText:        res.Text,
		Changes:             changes,
		Status:              int32(res.Status),
		ProcessingTimeMs:    res.Duration.Milliseconds(),
		CleaningTimeMs:      timings["normalize"],
		FillerTimeMs:        timings["disfluency"],
		VocabMatchTimeMs:    0, // lexnorm 内置在 alias 内（不单独计时）
		AliasTimeMs:         timings["alias"],
		DeterministicTimeMs: timings["deterministic"],
		PinyinTimeMs:        timings["pinyin"],
		FuzzyTimeMs:         timings["fuzzy_vocab"],
		ContextTimeMs:       timings["ctxproc"],
		ErrorMessage:        formatLexnormErrors(res.Errors),
	}, nil
}

// convertChanges 把 lexnorm.Change 转 v1.EnhanceChange（HTTP API 兼容层）。
//
// 字段映射：
//   - Action → action 字符串（"replace"/"remove"/"suggest"）
//   - From/To → from/to
//   - Confidence → confidence
//   - Reason → reason
//   - type：按 Source 名称映射（lexnorm 内置 processor 不填 Processor 字段）
//   - source：取 Change.Source（由各 processor 显式设置）
//   - locked：当前由 upstream processor 锁定的区间，lexnorm 未直接暴露
func convertChanges(cs []lexnorm.Change) []*v1.EnhanceChange {
	out := make([]*v1.EnhanceChange, 0, len(cs))
	for _, c := range cs {
		// type 推断：Source 通常是 processor 名（normalize/disfluency/alias/...）
		typ := changeTypeFromProcessor(c.Source, c.RuleID)

		// action 修正：disfluency / normalize 把 "" 作为 to 时是删除，但 lexnorm 标 ActionReplace
		action := c.Action.String()
		if c.To == "" && action == "replace" {
			action = "remove"
		}

		out = append(out, &v1.EnhanceChange{
			From:       c.From,
			To:         c.To,
			Action:     action,
			Type:       typ,
			Source:     c.Source,
			Confidence: float32(c.Confidence),
			Locked:     false,
			Reason:     c.Reason,
		})
	}
	return out
}

// changeTypeFromProcessor 把 Processor 名映射到 v1.EnhanceChange.Type。
//
// 旧 wire 用法：下游消费者按 type 分组（CLEAN/FILLER/ALIAS/CORRECTION/PINYIN/FUZZY/CONTEXT）。
func changeTypeFromProcessor(processor, ruleID string) string {
	switch processor {
	case "normalize":
		return "CLEAN"
	case "disfluency":
		return "FILLER"
	case "alias":
		return "ALIAS"
	case "deterministic":
		if ruleID == "phrase" {
			return "PHRASE"
		}
		return "CORRECTION"
	case "pinyin":
		return "PINYIN"
	case "fuzzy_vocab":
		return "FUZZY"
	case "ctxproc":
		return "CONTEXT"
	}
	return processor
}

// formatLexnormErrors 把 error 切片拼成单个错误消息。
func formatLexnormErrors(errs []error) string {
	if len(errs) == 0 {
		return ""
	}
	msg := ""
	for i, e := range errs {
		if i > 0 {
			msg += "; "
		}
		msg += e.Error()
	}
	return msg
}
