// Package processors · observer.go
// Observer 接口 + ObservingProcessor 装饰器（Decorator 模式 + Observer 模式）。
//
// 设计要点：
//   1. Observer 接口是统一的 pipeline + step 全生命周期事件入口
//   2. ObservingProcessor 装饰 inner TextProcessor，自动触发 observer
//   3. safeNotify 包装：observer 自身 panic 不影响主流程
//   4. observer 内不允许做重 I/O / 阻塞主流程
package processors

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Observer 监听 pipeline + processor 全生命周期事件。
//
// 实现规则（强约束）：
//   1. 所有方法必须线程安全（Pipeline 可能在不同 goroutine 调）
//   2. 方法内不允许 panic（safeNotify 会 recover，但污染日志）
//   3. 方法内不允许重 I/O（耗时操作放异步队列；或用 BufferedObserver）
//   4. 方法内不允许修改 ec / snapshot（只读）
//   5. OnProcessorComplete 的 changes 是该步骤产生的变更副本（不引用 ec.Changes）
type Observer interface {
	// Pipeline 级
	OnPipelineStart(ctx context.Context, processorNames []string)
	OnPipelineComplete(ctx context.Context, snapshot EnhancementSnapshot)
	OnPipelineError(ctx context.Context, err error)

	// Step 级
	OnProcessorStart(ctx context.Context, name string)
	OnProcessorComplete(ctx context.Context, name string, dur time.Duration, changes []Change)
	OnProcessorError(ctx context.Context, name string, err error)
}

// EnhancementSnapshot 是 ec 的不可变快照（observer 用）。
type EnhancementSnapshot struct {
	OriginalText string
	EnhancedText string
	Status       int32
	Canceled     bool
	Changes      []Change
	Timings      map[string]time.Duration
	LockedSpans  []string
	Errors       []error
	Duration     time.Duration
}

// SnapshotOf 从 ec 取不可变快照。
func SnapshotOf(ec *EnhancementContext) EnhancementSnapshot {
	if ec == nil {
		return EnhancementSnapshot{}
	}
	changes := ec.GetChanges()
	timings := ec.GetTimings()
	locked := append([]string(nil), ec.LockedSpans...)
	errs := append([]error(nil), ec.Errors...)

	// 计算该步骤新增的 changes（按时间区间）
	// 简化：返回当前所有 changes；observer 可根据 timing 推断区间
	_ = time.Now()

	return EnhancementSnapshot{
		OriginalText: ec.GetRawText(),
		EnhancedText: ec.GetText(),
		Status:       ec.GetStatus(),
		Canceled:     ec.Canceled,
		Changes:      changes,
		Timings:      timings,
		LockedSpans:  locked,
		Errors:       errs,
		Duration:     ec.Elapsed(),
	}
}

// NewObservingProcessor 构造装饰器。
func NewObservingProcessor(inner TextProcessor, observers []Observer) *ObservingProcessor {
	if inner == nil {
		panic("processors: ObservingProcessor.inner is nil")
	}
	return &ObservingProcessor{
		inner:     inner,
		observers: observers,
	}
}

// ObservingProcessor 是 Decorator：包装 inner processor + 注入 observer。
//
// 不持有任何 per-request 状态；observers 列表在构造时定下。
// 可被多 goroutine 共享。
type ObservingProcessor struct {
	inner     TextProcessor
	observers []Observer
}

// Name 转发到 inner。
func (o *ObservingProcessor) Name() string { return o.inner.Name() }

// Process 实现 textenhance.TextProcessor：跑 inner + 通知 observers。
//
// HA：
//   - inner panic → 记录到 OnProcessorError + re-panic（让 Pipeline 兜底）
//   - observer panic → safeNotify 吞掉，不影响主流程
func (o *ObservingProcessor) Process(ctx context.Context, ec *EnhancementContext) {
	// 记录 stepsChangesBaseline 用于 OnProcessorComplete
	baseline := len(ec.GetChanges())
	o.notifyStart(ctx)

	t0 := time.Now()
	innerPanic := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				innerPanic = true
				o.notifyError(ctx, fmt.Errorf("processor %s panic: %v", o.Name(), r))
			}
		}()
		o.inner.Process(ctx, ec)
	}()
	dur := time.Since(t0)

	// 计算本次步骤新增的 changes
	allChanges := ec.GetChanges()
	var newChanges []Change
	if len(allChanges) > baseline {
		newChanges = append(newChanges, allChanges[baseline:]...)
	}
	o.notifyComplete(ctx, dur, newChanges)

	// 重新 panic（让 Pipeline.runOneStep 兜底）
	if innerPanic {
		panic("re-panic from ObservingProcessor")
	}
}

func (o *ObservingProcessor) notifyStart(ctx context.Context) {
	for _, ob := range o.observers {
		ob := ob
		safeNotify(func() { ob.OnProcessorStart(ctx, o.Name()) }, "OnProcessorStart", o.Name())
	}
}

func (o *ObservingProcessor) notifyComplete(ctx context.Context, dur time.Duration, changes []Change) {
	for _, ob := range o.observers {
		ob := ob
		safeNotify(func() { ob.OnProcessorComplete(ctx, o.Name(), dur, changes) }, "OnProcessorComplete", o.Name())
	}
}

func (o *ObservingProcessor) notifyError(ctx context.Context, err error) {
	for _, ob := range o.observers {
		ob := ob
		safeNotify(func() { ob.OnProcessorError(ctx, o.Name(), err) }, "OnProcessorError", o.Name())
	}
}

// safeNotify 包装 observer 调用，panic 不影响主流程。
//
// HA：observer 自身 panic → recover + 返回 false；不向上传播。
func safeNotify(fn func(), name, target string) {
	defer func() {
		if r := recover(); r != nil {
			// 静默：observer panic 不应阻断主流程；调用方可加 logger 包装
			_ = r
		}
	}()
	fn()
}

// NotifyPipelineStart 通知所有 observer pipeline 开始。
func NotifyPipelineStart(ctx context.Context, observers []Observer, processorNames []string) {
	for _, ob := range observers {
		ob := ob
		safeNotify(func() { ob.OnPipelineStart(ctx, processorNames) }, "OnPipelineStart", "")
	}
}

// NotifyPipelineComplete 通知所有 observer pipeline 完成。
func NotifyPipelineComplete(ctx context.Context, observers []Observer, snapshot EnhancementSnapshot) {
	for _, ob := range observers {
		ob := ob
		safeNotify(func() { ob.OnPipelineComplete(ctx, snapshot) }, "OnPipelineComplete", "")
	}
}

// NotifyPipelineError 通知所有 observer pipeline 出错。
func NotifyPipelineError(ctx context.Context, observers []Observer, err error) {
	for _, ob := range observers {
		ob := ob
		safeNotify(func() { ob.OnPipelineError(ctx, err) }, "OnPipelineError", "")
	}
}

// 编译期断言
var (
	_ TextProcessor = (*ObservingProcessor)(nil)
	_ sync.Mutex     = sync.Mutex{}
)