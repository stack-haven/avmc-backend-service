// Package processors · context.go
// EnhancementContext：per-request 不可变状态。
//
// 设计要点：
//   1. 构造后 RawText / Vocab / Policy 不可变
//   2. Text / Changes / LockedSpans / Timings / Errors / Canceled 仅 append-only
//   3. 线程安全（每个请求独立一个 ec，但并发 append 仍需 mutex 保护）
//   4. Pipeline 与 Processor 共用此类型（互不依赖）
package processors

import (
	"context"
	"strings"
	"sync"
	"time"
)

// EnhancementContext per-request 状态。
type EnhancementContext struct {
	RawText     string                  // 原始文本（不可变）
	Text        string                  // 当前文本（processor 原地修改）
	Vocab       *VocabularySnapshot     // 不可变快照
	Policy      PolicyReader            // 不可变策略（接口，避免与 textenhance.Policy 循环）
	Changes     []Change                // append-only
	LockedSpans []string                // append-only
	Timings     map[string]time.Duration // processor name → duration
	Errors      []error                 // append-only
	Canceled    bool
	Status      int32 // 0=未运行
	startedAt   time.Time
	mu          sync.Mutex
}

// PolicyReader 是 processor 读 policy 的最小接口（不依赖 textenhance.Policy 具体类型）。
//
// textenhance.Policy 实现了 IsEnabled / 阈值字段，processor 通过 type assertion 访问。
type PolicyReader interface {
	IsEnabled(name string) bool
}

// NewEnhancementContext 构造 per-request ec。
func NewEnhancementContext(rawText string, vocab *VocabularySnapshot, policy PolicyReader) *EnhancementContext {
	if rawText == "" {
		rawText = ""
	}
	if vocab == nil {
		vocab = EmptyVocabularySnapshot()
	}
	return &EnhancementContext{
		RawText:   rawText,
		Text:      rawText,
		Vocab:     vocab,
		Policy:    policy,
		Timings:   make(map[string]time.Duration, 8),
		startedAt: time.Now(),
	}
}

// appendChange 线程安全追加 Change。
func (c *EnhancementContext) appendChange(ch Change) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Changes = append(c.Changes, ch)
}

// AppendChange 公开 API。
func (c *EnhancementContext) AppendChange(ch Change) { c.appendChange(ch) }

// appendLockedSpan 线程安全追加。
func (c *EnhancementContext) appendLockedSpan(span string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.LockedSpans = append(c.LockedSpans, span)
}

// appendError 线程安全追加（nil 跳过）。
func (c *EnhancementContext) appendError(err error) {
	if err == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Errors = append(c.Errors, err)
}

// AppendError 公开 API。
func (c *EnhancementContext) AppendError(err error) { c.appendError(err) }

// recordTiming 线程安全记录。
func (c *EnhancementContext) recordTiming(name string, dur time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Timings[name] = dur
}

// RecordTiming 公开 API。
func (c *EnhancementContext) RecordTiming(name string, dur time.Duration) {
	c.recordTiming(name, dur)
}

// Lock 标记片段锁定。
func (c *EnhancementContext) Lock(span string) { c.appendLockedSpan(span) }

// IsLocked 判断片段是否已锁定。
func (c *EnhancementContext) IsLocked(span string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, s := range c.LockedSpans {
		if s == span {
			return true
		}
	}
	return false
}

// Elapsed 总耗时。
func (c *EnhancementContext) Elapsed() time.Duration {
	return time.Since(c.startedAt)
}

// GetText 返回当前 Text。
func (c *EnhancementContext) GetText() string { return c.Text }

// GetRawText 返回原始 Text。
func (c *EnhancementContext) GetRawText() string { return c.RawText }

// SetText 设置当前 Text（processor 内部使用）。
func (c *EnhancementContext) SetText(s string) { c.Text = s }

// GetChanges 返回 Changes 副本。
func (c *EnhancementContext) GetChanges() []Change {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Change, len(c.Changes))
	copy(out, c.Changes)
	return out
}

// GetTimings 返回 Timings 副本。
func (c *EnhancementContext) GetTimings() map[string]time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]time.Duration, len(c.Timings))
	for k, v := range c.Timings {
		out[k] = v
	}
	return out
}

// GetStatus 返回当前 Status。
func (c *EnhancementContext) GetStatus() int32 { return c.Status }

// SetStatus 设置 Status（Pipeline 使用）。
func (c *EnhancementContext) SetStatus(s int32) { c.Status = s }

// JoinErrors 拼接所有 error 为字符串。
func (c *EnhancementContext) JoinErrors() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.Errors) == 0 {
		return ""
	}
	parts := make([]string, 0, len(c.Errors))
	for _, e := range c.Errors {
		parts = append(parts, e.Error())
	}
	return strings.Join(parts, "; ")
}

// 编译期断言
var _ = context.Background