// Package cleaning 实现文本清洗策略（第 1 步）。
//
// 算法与 evie/service/internal/biz/enhancement.go 的 TextCleaningStep 一致；
// 仅调整为 Functional Options 模式 + TextProcessor 接口。
package cleaning

import (
	"context"
	"regexp"

	"backend-service/pkg/textenhance/processors"
)

// Processor 文本清洗策略。
//
// 字段全部为构造时通过 Options 注入，构造后不可变。
// 正则编译一次后缓存；不在每请求重复编译（性能优化）。
type Processor struct {
	stripControlChars  bool
	collapseWhitespace bool
	dedupPunctuation   bool

	controlCharRe *regexp.Regexp
	whitespaceRe  *regexp.Regexp
	punctRe       *regexp.Regexp
}

// Option 配置 Processor 的函数（Functional Options 模式）。
type Option func(*Processor)

// WithStripControlChars 控制是否移除控制字符（\x00-\x1f、\x7f）。
func WithStripControlChars(enabled bool) Option {
	return func(p *Processor) { p.stripControlChars = enabled }
}

// WithCollapseWhitespace 控制是否合并连续空白。
func WithCollapseWhitespace(enabled bool) Option {
	return func(p *Processor) { p.collapseWhitespace = enabled }
}

// WithDedupPunctuation 控制是否去重连续标点。
func WithDedupPunctuation(enabled bool) Option {
	return func(p *Processor) { p.dedupPunctuation = enabled }
}

// NewTextCleaningProcessor 构造清洗策略。
//
// 默认值：stripControlChars / collapseWhitespace / dedupPunctuation 全部 true。
// 正则编译在此完成（构造一次性）。
func NewTextCleaningProcessor(opts ...Option) *Processor {
	p := &Processor{
		stripControlChars:  true,
		collapseWhitespace: true,
		dedupPunctuation:   true,
	}
	for _, opt := range opts {
		opt(p)
	}
	// 编译正则（按需；只编译启用的）
	if p.stripControlChars {
		p.controlCharRe = regexp.MustCompile(`[\x00-\x1f\x7f]`)
	}
	if p.collapseWhitespace {
		p.whitespaceRe = regexp.MustCompile(`\s+`)
	}
	if p.dedupPunctuation {
		p.punctRe = regexp.MustCompile(`[。，！？；：、,!.?;:]{2,}`)
	}
	return p
}

// Name 返回 processor 标识。
func (p *Processor) Name() string { return "cleaning" }

// Process 实现 processors.TextProcessor。
//
// 算法一字不改（从 evie/service 复制）。
func (p *Processor) Process(ctx context.Context, ec *processors.EnhancementContext) {
	if ec == nil {
		return
	}
	// ctx 检查：重活（replaceAll）前 select
	select {
	case <-ctx.Done():
		return
	default:
	}

	text := ec.Text
	if p.stripControlChars && p.controlCharRe != nil {
		text = p.controlCharRe.ReplaceAllString(text, "")
	}
	if p.collapseWhitespace && p.whitespaceRe != nil {
		text = p.whitespaceRe.ReplaceAllString(text, " ")
	}
	if p.dedupPunctuation && p.punctRe != nil {
		text = p.punctRe.ReplaceAllStringFunc(text, func(s string) string {
			return string([]rune(s)[0])
		})
	}

	if text == ec.Text {
		return // 无变化
	}
	ec.Text = text
	// 记录 changes（按 evie/service 风格拆分为多个 change 记录）
	// 简化：单条 change 描述整次清洗
	ec.Changes = append(ec.Changes, processors.Change{
		Action:     processors.ActionReplace,
		Type:       processors.TypeClean,
		Source:     processors.SourceSystem,
		Confidence: 1.0,
		Locked:     true,
		Reason:     "whitespace/control/punctuation cleanup",
	})
}

// 编译期断言
var _ processors.TextProcessor = (*Processor)(nil)