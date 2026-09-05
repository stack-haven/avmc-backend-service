// Package observers · logging.go
// LoggingObserver：把 pipeline / processor 事件写到 logger。
//
// 默认 level：
//   - Debug: PipelineStart / PipelineComplete / ProcessorStart / ProcessorComplete
//   - Warn:   ProcessorError / PipelineError
package observers

import (
	"context"
	"time"

	"backend-service/pkg/textenhance/processors"
)

// LoggingObserver 记录 pipeline 全生命周期到 logger。
type LoggingObserver struct {
	log Logger
}

// NewLoggingObserver 构造。
func NewLoggingObserver(log Logger) *LoggingObserver {
	if log == nil {
		log = DiscardLogger{}
	}
	return &LoggingObserver{log: log}
}

// OnPipelineStart 记录开始。
func (o *LoggingObserver) OnPipelineStart(ctx context.Context, names []string) {
	o.log.WithContext(ctx).Debugf("textenhance: pipeline start, %d processors: %v", len(names), names)
}

// OnPipelineComplete 记录结束。
func (o *LoggingObserver) OnPipelineComplete(ctx context.Context, snap processors.EnhancementSnapshot) {
	o.log.WithContext(ctx).Debugf(
		"textenhance: pipeline complete, status=%d(%s) text=%q→%q changes=%d errors=%d duration=%v",
		snap.Status, processors.StatusName(snap.Status),
		snap.OriginalText, snap.EnhancedText,
		len(snap.Changes), len(snap.Errors), snap.Duration,
	)
}

// OnPipelineError 记录 pipeline panic。
func (o *LoggingObserver) OnPipelineError(ctx context.Context, err error) {
	o.log.WithContext(ctx).Warnf("textenhance: pipeline error: %v", err)
}

// OnProcessorStart 记录步骤开始。
func (o *LoggingObserver) OnProcessorStart(ctx context.Context, name string) {
	o.log.WithContext(ctx).Debugf("textenhance:   step start: %s", name)
}

// OnProcessorComplete 记录步骤完成。
func (o *LoggingObserver) OnProcessorComplete(ctx context.Context, name string, dur time.Duration, changes []processors.Change) {
	o.log.WithContext(ctx).Debugf("textenhance:   step done:  %s dur=%v changes=%d", name, dur, len(changes))
}

// OnProcessorError 记录步骤错误（warn）。
func (o *LoggingObserver) OnProcessorError(ctx context.Context, name string, err error) {
	o.log.WithContext(ctx).Warnf("textenhance:   step error: %s err=%v", name, err)
}

// 编译期断言
var _ processors.Observer = (*LoggingObserver)(nil)