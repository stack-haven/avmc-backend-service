// Package pinyin_correction 实现拼音纠错策略（第 7 步，推断）。
//
// 算法与 evie/service/internal/biz/enhancement_inference.go 的 PinyinCorrectionStep 一致。
// 依赖 processors.PinyinService 接口注入（默认 processors.DefaultPinyinService）。
package pinyin_correction

import (
	"context"
	"sort"
	"strings"

	"backend-service/pkg/textenhance/processors"
)

// sortedRelationKeys 按字典序返回 Relations 的 key（确定性遍历）。
func sortedRelationKeys(vocab *processors.VocabularySnapshot) []string {
	keys := make([]string, 0, len(vocab.Relations))
	for k := range vocab.Relations {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Processor 拼音纠错策略。
type Processor struct {
	pinyinSvc         processors.PinyinService // 注入；默认 DefaultPinyinService
	autoThreshold     float64
	lockedConfidence  float64
}

// Option 配置 Processor 的函数。
type Option func(*Processor)

// WithPinyinService 注入 pinyin 服务（默认 DefaultPinyinService）。
//
// 用法：
//   pinyin_correction.NewPinyinCorrectionProcessor(
//       pinyin_correction.WithPinyinService(myMockPinyin),
//   )
func WithPinyinService(svc processors.PinyinService) Option {
	return func(p *Processor) {
		if svc != nil {
			p.pinyinSvc = svc
		}
	}
}

// WithAutoThreshold 设置自动替换阈值（默认 0.85）。
func WithAutoThreshold(v float64) Option {
	return func(p *Processor) {
		if v > 0 && v <= 1 {
			p.autoThreshold = v
		}
	}
}

// WithLockedConfidence 设置锁定置信度（默认 0.85）。
func WithLockedConfidence(v float64) Option {
	return func(p *Processor) {
		if v > 0 && v <= 1 {
			p.lockedConfidence = v
		}
	}
}

// NewPinyinCorrectionProcessor 构造拼音纠错策略。
func NewPinyinCorrectionProcessor(opts ...Option) *Processor {
	p := &Processor{
		pinyinSvc:        processors.NewDefaultPinyinService(),
		autoThreshold:    0.85,
		lockedConfidence: 0.85,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Name 返回 processor 标识。
func (p *Processor) Name() string { return "pinyin_correction" }

// Process 实现 textenhance.TextProcessor。
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
			if rel == nil || rel.RelationType != "HOMOPHONE" {
				continue
			}
			target := p.resolveTarget(ec.Vocab, rel)
			if target == "" {
				continue
			}
			if ec.IsLocked(relatedText) {
				continue
			}
			if !strings.Contains(ec.Text, relatedText) {
				continue
			}
			before := ec.Text
			ec.Text = strings.ReplaceAll(ec.Text, relatedText, target)
			if ec.Text != before {
				ec.Changes = append(ec.Changes, processors.Change{
					From:       relatedText,
					To:         target,
					Action:     processors.ActionReplace,
					Type:       processors.TypePinyin,
					Source:     processors.SourceTenantDict,
					Confidence: p.lockedConfidence,
					Locked:     false,
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