// Package observers · counting.go
// CountingObserver：统计每步调用次数 / 错误数 / 总耗时。
//
// 用途：
//   - 调试（看哪个 processor 被调用最频繁 / 哪个最慢）
//   - 监控（导出为 metrics；M6b 之后可加 PrometheusObserver）
//   - 性能优化（按 stats 调阈值）
package observers

import (
	"context"
	"sync"
	"time"

	"backend-service/pkg/textenhance/processors"
)

// ProcessorStats 单个 processor 的统计。
type ProcessorStats struct {
	Invocations int64         // 完整执行次数（成功 + 失败）
	Errors      int64         // 错误次数
	Successes   int64         // 成功次数
	TotalTime   time.Duration // 累计执行时间
	MaxTime     time.Duration // 最长单次执行
}

// AvgTime 平均执行时间。
func (s *ProcessorStats) AvgTime() time.Duration {
	if s.Invocations == 0 {
		return 0
	}
	return s.Duration() / time.Duration(s.Invocations)
}

// Duration 累计时间（别名，兼容 metrics 接口）。
func (s *ProcessorStats) Duration() time.Duration {
	return s.TotalTime
}

// PipelineStats pipeline 整体统计。
type PipelineStats struct {
	Invocations int64
	Errors      int64
	Successes   int64
	TotalTime   time.Duration
}

// CountingObserver 线程安全统计。
type CountingObserver struct {
	mu            sync.RWMutex
	processorStats map[string]*ProcessorStats
	pipelineStats  *PipelineStats
}

// NewCountingObserver 构造。
func NewCountingObserver() *CountingObserver {
	return &CountingObserver{
		processorStats: make(map[string]*ProcessorStats),
		pipelineStats:  &PipelineStats{},
	}
}

// OnPipelineStart pipeline 开始。
func (o *CountingObserver) OnPipelineStart(ctx context.Context, names []string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.pipelineStats.Invocations++
}

// OnPipelineComplete pipeline 成功完成。
func (o *CountingObserver) OnPipelineComplete(ctx context.Context, snap processors.EnhancementSnapshot) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.pipelineStats.Successes++
	o.pipelineStats.TotalTime += snap.Duration
}

// OnPipelineError pipeline 错误。
func (o *CountingObserver) OnPipelineError(ctx context.Context, err error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.pipelineStats.Errors++
}

// OnProcessorStart 步骤开始。
func (o *CountingObserver) OnProcessorStart(ctx context.Context, name string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if _, ok := o.processorStats[name]; !ok {
		o.processorStats[name] = &ProcessorStats{}
	}
	o.processorStats[name].Invocations++
}

// OnProcessorComplete 步骤完成（成功）。
func (o *CountingObserver) OnProcessorComplete(ctx context.Context, name string, dur time.Duration, changes []processors.Change) {
	o.mu.Lock()
	defer o.mu.Unlock()
	s, ok := o.processorStats[name]
	if !ok {
		s = &ProcessorStats{}
		o.processorStats[name] = s
	}
	s.Successes++
	s.TotalTime += dur
	if dur > s.MaxTime {
		s.MaxTime = dur
	}
}

// OnProcessorError 步骤错误。
func (o *CountingObserver) OnProcessorError(ctx context.Context, name string, err error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if _, ok := o.processorStats[name]; !ok {
		o.processorStats[name] = &ProcessorStats{}
	}
	o.processorStats[name].Errors++
}

// Stats 返回 processor 统计快照（副本）。
func (o *CountingObserver) Stats() map[string]ProcessorStats {
	o.mu.RLock()
	defer o.mu.RUnlock()
	out := make(map[string]ProcessorStats, len(o.processorStats))
	for k, v := range o.processorStats {
		out[k] = *v
	}
	return out
}

// PipelineStatsSnapshot 返回 pipeline 统计。
func (o *CountingObserver) PipelineStatsSnapshot() PipelineStats {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return *o.pipelineStats
}

// 编译期断言
var _ processors.Observer = (*CountingObserver)(nil)