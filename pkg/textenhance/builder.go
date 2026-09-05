// Package textenhance · builder.go
// BuildPipeline：从 Registry + Policy + 可选 Observers 一行构建 Pipeline。
//
// 用法：
//   pipeline, err := textenhance.BuildPipeline(reg, policy,
//       textenhance.WithObservers(loggingObs, countingObs),
//   )
package textenhance

import (
	"fmt"

	"backend-service/pkg/textenhance/processors"
)

// BuilderOption BuildPipeline 配置函数。
type BuilderOption func(*builderConfig)

type builderConfig struct {
	observers []processors.Observer
}

// WithObservers 注入 Observer 列表（M6b：Decorator 自动包裹 processor）。
//
// 所有 observer 会在 pipeline.start / per-processor / pipeline.complete 时收到事件。
func WithObservers(observers ...processors.Observer) BuilderOption {
	return func(c *builderConfig) {
		c.observers = append(c.observers, observers...)
	}
}

// BuildPipeline 根据 policy 选 processor 列表 → 调 Registry 构造 → 可选 Decorator 包裹 → Pipeline。
func BuildPipeline(reg *Registry, policy *Policy, opts ...BuilderOption) (*Pipeline, error) {
	if reg == nil {
		return nil, fmt.Errorf("textenhance: registry is nil")
	}
	cfg := &builderConfig{}
	for _, opt := range opts {
		opt(cfg)
	}
	validated, _ := policy.Validate()

	procs := make([]processors.TextProcessor, 0, len(DefaultProcessorOrder)+1)
	for _, name := range validated.EnabledOrder() {
		proc, err := reg.Build(name)
		if err != nil {
			return nil, err
		}
		// Decorator 包裹（如果有 observer）
		if len(cfg.observers) > 0 {
			proc = processors.NewObservingProcessor(proc, cfg.observers)
		}
		procs = append(procs, proc)
	}

	if validated.LLMEnabled {
		proc, err := reg.Build("llm_reserved")
		if err == nil {
			if len(cfg.observers) > 0 {
				proc = processors.NewObservingProcessor(proc, cfg.observers)
			}
			procs = append(procs, proc)
		}
	}

	return NewPipeline(procs, validated, cfg.observers...), nil
}