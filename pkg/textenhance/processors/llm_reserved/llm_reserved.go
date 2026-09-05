// Package llm_reserved 实现 LLM 步骤占位（第 10 步，保留位，默认 no-op）。
//
// 设计要点：
//   1. 默认 NewLLMReservedProcessor() 是 no-op（Process 直接返回）
//   2. 仅在 Policy.LLMEnabled=true 时被 BuildPipeline 包含
//   3. 未来接入 LLM：在 Process 中通过 Option 注入的 client 发请求
package llm_reserved

import (
	"context"

	"backend-service/pkg/textenhance/processors"
)

// LLMClient 抽象接口（未来注入 OpenAI / Claude / 本地模型等）。
type LLMClient interface {
	Complete(ctx context.Context, prompt string) (string, error)
}

// Processor LLM 步骤占位。
type Processor struct {
	client   LLMClient // nil = no-op
	prompt   string    // 提示词模板
	enabled  bool      // false = 永不调 LLM
}

// Option 配置 Processor 的函数。
type Option func(*Processor)

// WithClient 注入 LLM client（不传 = no-op）。
func WithClient(c LLMClient) Option {
	return func(p *Processor) { p.client = c }
}

// WithPrompt 设置提示词模板。
func WithPrompt(s string) Option {
	return func(p *Processor) { p.prompt = s }
}

// WithEnabled 显式启用（默认 false / no-op）。
func WithEnabled(enabled bool) Option {
	return func(p *Processor) { p.enabled = enabled }
}

// NewLLMReservedProcessor 构造 LLM 步骤占位。
//
// 默认：client=nil, enabled=false → Process 直接返回（不修改 text）。
func NewLLMReservedProcessor(opts ...Option) *Processor {
	p := &Processor{
		client:  nil,
		prompt:  "",
		enabled: false,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Name 返回 processor 标识。
func (p *Processor) Name() string { return "llm_reserved" }

// Process 实现 processors.TextProcessor。
//
// 当前为 no-op（不调任何外部资源）。
// 未来接入 LLM 时的预期行为：
//   1. 检查 enabled + client 是否为 nil
//   2. 构造 prompt（含 ec.Text + ec.Changes 上下文）
//   3. 调 client.Complete
//   4. 如返回非空 → 替换 ec.Text，记录 change
//   5. error → 累积到 ec.Errors
func (p *Processor) Process(ctx context.Context, ec *processors.EnhancementContext) {
	if ec == nil {
		return
	}
	// HA: ctx 检查
	select {
	case <-ctx.Done():
		return
	default:
	}

	if !p.enabled || p.client == nil {
		// 默认 no-op：什么都不做
		return
	}

	// 未来 LLM 接入位置（当前 stub）
	// result, err := p.client.Complete(ctx, ec.Text)
	// if err != nil {
	//     ec.appendError(fmt.Errorf("llm_reserved: %w", err))
	//     return
	// }
	// if result != "" && result != ec.Text {
	//     ec.Changes = append(ec.Changes, processors.Change{
	//         From: ec.Text, To: result,
	//         Action: processors.ActionReplace,
	//         Type:   textenhance.TypeLLM,  // 未来加
	//         Source: processors.SourceLLM,
	//         Confidence: 0.0,  // LLM 不给置信度
	//         Locked: false,
	//     })
	//     ec.Text = result
	// }
}

// 编译期断言
var _ processors.TextProcessor = (*Processor)(nil)