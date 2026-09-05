// Package alias_resolution 实现别名解析策略（第 4 步）。
//
// 算法与 evie/service/internal/biz/enhancement.go 的 AliasResolutionStep 一致。
package alias_resolution

import (
	"context"
	"sort"
	"strings"

	"backend-service/pkg/textenhance/processors"
)

// Processor 别名解析策略。
type Processor struct {
	maxPasses int // 最大扫描次数（防御性，避免死循环）
}

// Option 配置 Processor 的函数。
type Option func(*Processor)

// WithMaxPasses 设置最大扫描次数（默认 1）。
func WithMaxPasses(n int) Option {
	return func(p *Processor) {
		if n > 0 { p.maxPasses = n }
	}
}

// NewAliasResolutionProcessor 构造别名解析策略。
func NewAliasResolutionProcessor(opts ...Option) *Processor {
	p := &Processor{maxPasses: 1}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Name 返回 processor 标识。
func (p *Processor) Name() string { return "alias_resolution" }

// Process 实现 processors.TextProcessor。
//
// 算法（与 evie/service AliasResolutionStep 一致）：
//   1. 遍历 ec.Vocab.Relations，匹配 ALIAS 类型
//   2. 找到 related_text → target 映射
//   3. 在 text 中替换；action=RESOLVE，locked=true
func (p *Processor) Process(ctx context.Context, ec *processors.EnhancementContext) {
	if ec == nil || ec.Vocab == nil {
		return
	}
	select {
	case <-ctx.Done():
		return
	default:
	}

	changed := true
	passes := 0
	for changed && passes < p.maxPasses {
		changed = false
		passes++

		// 确定性顺序：relatedText 排序遍历，避免 map 随机迭代产生不一致输出。
		keys := make([]string, 0, len(ec.Vocab.Relations))
		for k := range ec.Vocab.Relations {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, relatedText := range keys {
			rels := ec.Vocab.Relations[relatedText]
			for _, rel := range rels {
				if rel.RelationType != "ALIAS" {
					continue
				}
				target := p.resolveTarget(ec.Vocab, rel)
				if target == "" || !strings.Contains(ec.Text, relatedText) {
					continue
				}
				// 已被锁定则跳过
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
						Action:     processors.ActionResolve,
						Type:       processors.TypeAlias,
						Source:     processors.SourceTenantDict,
						Confidence: 1.0,
						Locked:     true,
					})
					changed = true
				}
			}
		}
	}
}

// resolveTarget 解析关系的目标标准词。
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