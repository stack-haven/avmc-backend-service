// Package textenhance · pipeline.go
// Pipeline：编排器（按 policy 顺序执行 TextProcessor 列表）。
//
// HA 三重保护：
//   1. defer recover：Processor 内部 panic 不挂 Pipeline
//   2. ctx 超时检查：每步前 select ctx.Done()
//   3. 错误累积：errors 不阻断，仅累积到 ec.Errors
//
// Observer 集成（M6b）：
//   - Pipeline 接受 []processors.Observer
//   - 构造时计算 processor names 列表
//   - OnPipelineStart / OnPipelineComplete / OnPipelineError 在 Run 中触发
//   - OnProcessorStart / OnProcessorComplete / OnProcessorError 由 ObservingProcessor 触发
package textenhance

import (
	"context"
	"fmt"
	"time"

	"backend-service/pkg/textenhance/processors"
)

// defaultPipelineTimeout Pipeline 默认超时（调用方可用 ctx 覆盖）。
const defaultPipelineTimeout = 5 * time.Second

// Pipeline 持有不可变的 processor 列表 + observers。
type Pipeline struct {
	processors []processors.TextProcessor
	policy     *Policy
	observers  []processors.Observer
	procNames  []string // 缓存的 processor 名称列表（用于 OnPipelineStart）
}

// NewPipeline 构造 Pipeline（不推荐直接调用；用 BuildPipeline）。
func NewPipeline(procs []processors.TextProcessor, policy *Policy, observers ...processors.Observer) *Pipeline {
	names := make([]string, 0, len(procs))
	for _, p := range procs {
		names = append(names, p.Name())
	}
	return &Pipeline{
		processors: procs,
		policy:     policy,
		observers:  observers,
		procNames:  names,
	}
}

// Processors 返回构造时的 processor 列表（仅读）。
func (p *Pipeline) Processors() []processors.TextProcessor { return p.processors }

// Observers 返回 observer 列表（仅读）。
func (p *Pipeline) Observers() []processors.Observer { return p.observers }

// Policy 返回构造时的 policy（仅读）。
func (p *Pipeline) Policy() *Policy { return p.policy }

// Run 顺序执行所有启用的 processor。
//
// 事件触发顺序：
//   1. OnPipelineStart（defer safeNotify）
//   2. 每个 processor: OnProcessorStart → Process → OnProcessorComplete
//   3. OnPipelineComplete（成功时）
//   4. OnPipelineError（panic 时）
func (p *Pipeline) Run(ctx context.Context, ec *EnhancementContext) {
	// HA-1: 整体 defer recover + OnPipelineError
	defer func() {
		if r := recover(); r != nil {
			ec.AppendError(fmt.Errorf("pipeline panic: %v", r))
			ec.SetStatus(StatusPanic)
			processors.NotifyPipelineError(ctx, p.observers, fmt.Errorf("pipeline panic: %v", r))
		}
		p.finalize(ec)
		if ec.GetStatus() == StatusSuccess || ec.GetStatus() == StatusPartial || ec.GetStatus() == StatusCanceled {
			snap := processors.SnapshotOf(ec)
			processors.NotifyPipelineComplete(ctx, p.observers, snap)
		}
	}()

	// HA-2: 默认超时
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultPipelineTimeout)
		defer cancel()
	}

	if ec == nil {
		return
	}

	// OnPipelineStart
	processors.NotifyPipelineStart(ctx, p.observers, p.procNames)

	for _, proc := range p.processors {
		if ctx.Err() != nil {
			ec.Canceled = true
			break
		}
		if !p.policy.IsEnabled(proc.Name()) {
			continue
		}
		p.runOneStep(ctx, proc, ec)
	}
}

// runOneStep 单步执行（panic recover + 计时）。
func (p *Pipeline) runOneStep(ctx context.Context, proc processors.TextProcessor, ec *EnhancementContext) {
	t0 := time.Now()
	defer func() {
		ec.RecordTiming(proc.Name(), time.Since(t0))
		if r := recover(); r != nil {
			ec.AppendError(fmt.Errorf("processor %s panic: %v", proc.Name(), r))
		}
	}()
	proc.Process(ctx, ec)
}

// finalize 终态计算。
func (p *Pipeline) finalize(ec *EnhancementContext) {
	switch ec.GetStatus() {
	case StatusPanic, StatusFailed:
		return
	}
	if ec.Canceled {
		ec.SetStatus(StatusCanceled)
		return
	}
	if len(ec.Errors) > 0 {
		ec.SetStatus(StatusPartial)
		return
	}
	ec.SetStatus(StatusSuccess)
}