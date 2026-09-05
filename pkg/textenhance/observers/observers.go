// Package observers 提供 textenhance 的默认 Observer 实现。
//
// 单独成包是为了避免 import cycle（observers 依赖 processors/Observer；
// processors 不依赖 observers；textenhance 根可选地同时使用两者）。
package observers

import "context"

// Logger 接口（与 kratos 解耦；observers 接受任何最小 logger 接口）。
//
// 含 WithContext：与 kratos log.Helper.WithContext 兼容。
type Logger interface {
	WithContext(ctx context.Context) Logger
	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
}

// DiscardLogger 静默 logger（测试 / 不需要日志场景）。
type DiscardLogger struct{}

func (DiscardLogger) WithContext(context.Context) Logger { return DiscardLogger{} }
func (DiscardLogger) Debugf(string, ...any)              {}
func (DiscardLogger) Infof(string, ...any)               {}
func (DiscardLogger) Warnf(string, ...any)               {}
func (DiscardLogger) Errorf(string, ...any)              {}

// StdLogger 标准 log 适配器（兼容 log.Logger）。
type StdLogger struct {
	Debug func(format string, args ...any)
	Info  func(format string, args ...any)
	Warn  func(format string, args ...any)
	Error func(format string, args ...any)
}

func (l StdLogger) WithContext(context.Context) Logger { return l }
func (l StdLogger) Debugf(format string, args ...any) {
	if l.Debug != nil { l.Debug(format, args...) }
}
func (l StdLogger) Infof(format string, args ...any) {
	if l.Info != nil { l.Info(format, args...) }
}
func (l StdLogger) Warnf(format string, args ...any) {
	if l.Warn != nil { l.Warn(format, args...) }
}
func (l StdLogger) Errorf(format string, args ...any) {
	if l.Error != nil { l.Error(format, args...) }
}

// 编译期断言
var (
	_ Logger = DiscardLogger{}
	_ Logger = StdLogger{}
)