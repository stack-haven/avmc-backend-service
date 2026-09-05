// Package context_correction 实现上下文纠错策略（第 9 步，推断）。
//
// 算法与 evie/service/internal/biz/enhancement_inference.go 的 ContextCorrectionStep 一致。
package context_correction

import (
	"context"
	"strings"

	"backend-service/pkg/textenhance/processors"
)

// ContextRule 上下文规则（from + context → to）。
type ContextRule struct {
	From    string // 候选原文
	Context string // 触发条件（必须出现）
	To      string // 替换目标
}

// Processor 上下文纠错策略。
type Processor struct {
	rules []ContextRule
}

// Option 配置 Processor 的函数。
type Option func(*Processor)

// WithRules 设置规则列表（覆盖默认）。
func WithRules(rules []ContextRule) Option {
	return func(p *Processor) {
		if len(rules) > 0 {
			p.rules = append([]ContextRule(nil), rules...)
		}
	}
}

// WithAddRule 追加单条规则。
func WithAddRule(from, context, to string) Option {
	return func(p *Processor) {
		p.rules = append(p.rules, ContextRule{From: from, Context: context, To: to})
	}
}

// NewContextCorrectionProcessor 构造上下文纠错策略。
func NewContextCorrectionProcessor(opts ...Option) *Processor {
	p := &Processor{
		rules: []ContextRule{
			{From: "功课", Context: "技术难点", To: "攻克"},
		},
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Name 返回 processor 标识。
func (p *Processor) Name() string { return "context_correction" }

// Process 实现 processors.TextProcessor。
//
// 算法（与 evie/service ContextCorrectionStep 一致）：
//   1. 遍历规则
//   2. text 含 rule.From 且含 rule.Context → 替换
//   3. action=REPLACE，confidence=0.9，locked=false（推断）
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
		if !strings.Contains(ec.Text, rule.Context) {
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
				Type:       processors.TypeContext,
				Source:     processors.SourceSystem,
				Confidence: 0.9,
				Locked:     false,
			})
		}
	}
}

// 编译期断言
var _ processors.TextProcessor = (*Processor)(nil)