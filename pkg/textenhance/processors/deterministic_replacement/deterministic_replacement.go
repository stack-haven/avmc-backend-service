// Package deterministic_replacement 实现确定性替换策略（第 5 步）。
//
// 算法与 evie/service/internal/biz/enhancement.go 的 DeterministicReplacementStep 一致。
package deterministic_replacement

import (
	"context"
	"sort"
	"strings"

	"backend-service/pkg/textenhance/processors"
)

// sortedRelationKeys 按字典序返回 ec.Vocab.Relations 的 key 列表（确定性遍历）。
func sortedRelationKeys(vocab *processors.VocabularySnapshot) []string {
	keys := make([]string, 0, len(vocab.Relations))
	for k := range vocab.Relations {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Processor 确定性替换策略。
type Processor struct{}

// Option 配置 Processor 的函数。
type Option func(*Processor)

// NewDeterministicReplacementProcessor 构造确定性替换策略（无 Options）。
func NewDeterministicReplacementProcessor(opts ...Option) *Processor {
	return &Processor{}
}

// Name 返回 processor 标识。
func (p *Processor) Name() string { return "deterministic_replacement" }

// Process 实现 processors.TextProcessor。
//
// 算法（与 evie/service DeterministicReplacementStep 一致）：
//   1. 遍历 ec.Vocab.Relations，匹配 CORRECTION 类型
//   2. 替换；action=REPLACE，locked=true
func (p *Processor) Process(ctx context.Context, ec *processors.EnhancementContext) {
	if ec == nil || ec.Vocab == nil {
		return
	}
	select {
	case <-ctx.Done():
		return
	default:
	}

	for _, relatedText := range sortedRelationKeys(ec.Vocab) {
		for _, rel := range ec.Vocab.Relations[relatedText] {
			if rel.RelationType != "CORRECTION" {
				continue
			}
			target := p.resolveTarget(ec.Vocab, rel)
			if target == "" || !strings.Contains(ec.Text, relatedText) {
				continue
			}
			if ec.IsLocked(relatedText) {
				continue
			}
			before := ec.Text
			ec.Text = strings.ReplaceAll(ec.Text, relatedText, target)
			if ec.Text != before {
				ec.Lock(target)
				ec.Changes = append(ec.Changes, processors.Change{
					From:       relatedText,
					To:         target,
					Action:     processors.ActionReplace,
					Type:       processors.TypeCorrection,
					Source:     processors.SourceTenantDict,
					Confidence: 1.0,
					Locked:     true,
				})
			}
		}
	}
}

func (p *Processor) resolveTarget(vocab *processors.VocabularySnapshot, rel *processors.VocabularyRelation) string {
	if rel == nil || rel.TargetEntryID == 0 {
		return ""
	}
	for _, e := range vocab.Entries {
		if e.ID == rel.TargetEntryID {
			return e.StandardText
		}
	}
	return ""
}

// 编译期断言
var _ processors.TextProcessor = (*Processor)(nil)