// Package phrase_standardization 实现短语标准化策略（第 6 步，夹在确定性与推断之间）。
//
// 算法与 evie/service/internal/biz/enhancement_inference.go 的 PhraseStandardizationStep 一致。
package phrase_standardization

import (
	"context"
	"strings"

	"backend-service/pkg/textenhance/processors"
)

// PhraseRule 短语规则（from → to）。
type PhraseRule struct {
	From string
	To   string
}

// Processor 短语标准化策略。
type Processor struct {
	rules []PhraseRule
}

// Option 配置 Processor 的函数。
type Option func(*Processor)

// WithRules 设置短语规则列表（覆盖默认）。
func WithRules(rules []PhraseRule) Option {
	return func(p *Processor) {
		if len(rules) > 0 {
			p.rules = append([]PhraseRule(nil), rules...)
		}
	}
}

// WithAddRule 追加单条规则。
func WithAddRule(from, to string) Option {
	return func(p *Processor) {
		p.rules = append(p.rules, PhraseRule{From: from, To: to})
	}
}

// NewPhraseStandardizationProcessor 构造短语标准化策略。
func NewPhraseStandardizationProcessor(opts ...Option) *Processor {
	p := &Processor{
		rules: []PhraseRule{
			{From: "个种籽", To: "颗种籽"},
			{From: "个种子", To: "颗种子"},
		},
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Name 返回 processor 标识。
func (p *Processor) Name() string { return "phrase_standardization" }

// Process 实现 processors.TextProcessor。
func (p *Processor) Process(ctx context.Context, ec *processors.EnhancementContext) {
	if ec == nil {
		return
	}
	select {
	case <-ctx.Done():
		return
	default:
	}

	for _, rule := range p.rules {
		if !strings.Contains(ec.Text, rule.From) {
			continue
		}
		if ec.IsLocked(rule.From) {
			continue
		}
		before := ec.Text
		ec.Text = strings.ReplaceAll(ec.Text, rule.From, rule.To)
		if ec.Text != before {
			ec.Lock(rule.To)
			ec.Changes = append(ec.Changes, processors.Change{
				From:       rule.From,
				To:         rule.To,
				Action:     processors.ActionReplace,
				Type:       processors.TypePhrase,
				Source:     processors.SourceSystem,
				Confidence: 1.0,
				Locked:     true,
			})
		}
	}
}

// 编译期断言
var _ processors.TextProcessor = (*Processor)(nil)