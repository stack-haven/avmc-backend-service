// Package metrics 提供轻量的 Prometheus 兼容指标收集与文本导出。
//
// 设计目标：
//   - 零外部依赖：仅使用标准库；
//   - 与 Prometheus exposition 文本格式兼容，便于接入现成监控；
//   - 支持带 label 的 Counter 与 Histogram；
//   - 线程安全：每个 label 组合使用原子 + 互斥，最小化热点竞争。
//
// 暴露：
//   - Default：进程级默认注册表；
//   - HTTPRequestsTotal / RequestDuration：HTTP 请求计数与延迟；
//   - ASRRequestsTotal / ASRDuration：ASR provider 调用计数与延迟；
//   - VocabSyncTotal：词库同步调用计数。
package metrics

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// makeKey 将 label 值列表拼成稳定的 key。空值或缺值以 "" 表示。
func makeKey(labelValues []string) string {
	if len(labelValues) == 0 {
		return ""
	}
	return strings.Join(labelValues, "\x00")
}

// splitKey 还原 label 值列表。
func splitKey(key string, n int) []string {
	if n == 0 {
		return nil
	}
	if key == "" {
		out := make([]string, n)
		return out
	}
	return strings.Split(key, "\x00")
}

// CounterVec 带 label 的累计计数器。
type CounterVec struct {
	name       string
	help       string
	labelNames []string
	store      sync.Map // key: string -> *uint64
}

// NewCounterVec 构造一个 CounterVec。
func NewCounterVec(name, help string, labelNames ...string) *CounterVec {
	return &CounterVec{name: name, help: help, labelNames: labelNames}
}

// Inc +1。
func (c *CounterVec) Inc(labelValues ...string) { c.Add(1, labelValues...) }

// Add 累加 v。
func (c *CounterVec) Add(v uint64, labelValues ...string) {
	if len(labelValues) != len(c.labelNames) {
		// 防御：label 不匹配时退化为 empty key。
		labelValues = make([]string, len(c.labelNames))
	}
	key := makeKey(labelValues)
	actual, _ := c.store.LoadOrStore(key, new(uint64))
	atomic.AddUint64(actual.(*uint64), v)
}

// HistogramVec 带 label 的 Histogram。
type HistogramVec struct {
	name       string
	help       string
	labelNames []string
	buckets    []float64 // upper bounds, sorted ascending
	store      sync.Map  // key: string -> *histState
}

// histState 单 label 集合的直方图状态。
type histState struct {
	mu     sync.Mutex
	counts []uint64 // len(buckets)+1，末位为 +Inf
	sum    float64
	count  uint64
}

// NewHistogramVec 构造一个 HistogramVec。
//
// buckets 必须按升序排好；不会拷贝，调用者请勿修改。
func NewHistogramVec(name, help string, buckets []float64, labelNames ...string) *HistogramVec {
	cp := make([]float64, len(buckets))
	copy(cp, buckets)
	return &HistogramVec{name: name, help: help, buckets: cp, labelNames: labelNames}
}

// Observe 记录一次观测值（秒）。
func (h *HistogramVec) Observe(v float64, labelValues ...string) {
	if len(labelValues) != len(h.labelNames) {
		labelValues = make([]string, len(h.labelNames))
	}
	key := makeKey(labelValues)
	actual, _ := h.store.LoadOrStore(key, &histState{counts: make([]uint64, len(h.buckets)+1)})
	state := actual.(*histState)
	state.mu.Lock()
	defer state.mu.Unlock()
	for i, b := range h.buckets {
		if v <= b {
			state.counts[i]++
		}
	}
	state.counts[len(h.buckets)]++ // +Inf
	state.sum += v
	state.count++
}

// Registry 一组指标的集合。
type Registry struct {
	mu         sync.Mutex
	counters   []*CounterVec
	histograms []*HistogramVec
}

// New 构造一个空 Registry。
func New() *Registry { return &Registry{} }

// RegisterCounter 注册一个 CounterVec。
func (r *Registry) RegisterCounter(c *CounterVec) {
	r.mu.Lock()
	r.counters = append(r.counters, c)
	r.mu.Unlock()
}

// RegisterHistogram 注册一个 HistogramVec。
func (r *Registry) RegisterHistogram(h *HistogramVec) {
	r.mu.Lock()
	r.histograms = append(r.histograms, h)
	r.mu.Unlock()
}

// Handler 返回 Prometheus 文本格式的 http.Handler。
func (r *Registry) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		r.writeTo(w)
	})
}

// writeTo 写出指标（可测试）。
func (r *Registry) writeTo(w io.Writer) {
	r.mu.Lock()
	counters := append([]*CounterVec(nil), r.counters...)
	histograms := append([]*HistogramVec(nil), r.histograms...)
	r.mu.Unlock()

	for _, c := range counters {
		fmt.Fprintf(w, "# HELP %s %s\n", c.name, c.help)
		fmt.Fprintf(w, "# TYPE %s counter\n", c.name)
		c.store.Range(func(k, v any) bool {
			values := splitKey(k.(string), len(c.labelNames))
			count := atomic.LoadUint64(v.(*uint64))
			fmt.Fprintf(w, "%s%s %d\n", c.name, formatLabels(c.labelNames, values), count)
			return true
		})
	}
	for _, h := range histograms {
		fmt.Fprintf(w, "# HELP %s %s\n", h.name, h.help)
		fmt.Fprintf(w, "# TYPE %s histogram\n", h.name)
		h.store.Range(func(k, v any) bool {
			values := splitKey(k.(string), len(h.labelNames))
			state := v.(*histState)
			state.mu.Lock()
			defer state.mu.Unlock()
			for i, b := range h.buckets {
				fmt.Fprintf(w, "%s_bucket%s %d\n", h.name, formatLabelsWithLE(h.labelNames, values, b), state.counts[i])
			}
			fmt.Fprintf(w, "%s_bucket%s %d\n", h.name, formatLabelsWithLE(h.labelNames, values, +mathInf()), state.counts[len(h.buckets)])
			fmt.Fprintf(w, "%s_sum%s %g\n", h.name, formatLabels(h.labelNames, values), state.sum)
			fmt.Fprintf(w, "%s_count%s %d\n", h.name, formatLabels(h.labelNames, values), state.count)
			return true
		})
	}
}

// mathInf 表示 +Inf。
func mathInf() float64 { return math.Inf(1) }

// formatLabels 渲染 {name="value",...} 形式；空时返回 ""。
func formatLabels(names, values []string) string {
	if len(names) == 0 {
		return ""
	}
	parts := make([]string, 0, len(names))
	for i, n := range names {
		parts = append(parts, fmt.Sprintf(`%s="%s"`, n, escapeLabelValue(values[i])))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// formatLabelsWithLE 在已有 label 基础上追加 le="x" 维度。
func formatLabelsWithLE(names, values []string, le float64) string {
	if len(names) == 0 {
		return fmt.Sprintf(`{le="%s"}`, formatFloat(le))
	}
	parts := make([]string, 0, len(names)+1)
	for i, n := range names {
		parts = append(parts, fmt.Sprintf(`%s="%s"`, n, escapeLabelValue(values[i])))
	}
	parts = append(parts, fmt.Sprintf(`le="%s"`, formatFloat(le)))
	return "{" + strings.Join(parts, ",") + "}"
}

// formatFloat 格式化 bucket 上限；+Inf 用 "Inf"。
func formatFloat(v float64) string {
	if v >= 1e200 {
		return "+Inf"
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%g", v), "0"), ".")
}

// escapeLabelValue 简单转义反斜杠、双引号、换行。
func escapeLabelValue(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)
	return r.Replace(s)
}

// ---- Default Registry & Pre-registered Metrics ----

// Default 默认注册表。
var Default = New()

// HTTPRequestsTotal HTTP 请求计数。
var HTTPRequestsTotal = NewCounterVec(
	"evie_http_requests_total",
	"Total HTTP requests handled by evie/tool.",
	"method", "path", "status",
)

// RequestDuration HTTP 请求延迟（秒）。
var RequestDuration = NewHistogramVec(
	"evie_http_request_duration_seconds",
	"HTTP request latency in seconds.",
	[]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	"method", "path",
)

// ASRRequestsTotal ASR provider 请求计数。
var ASRRequestsTotal = NewCounterVec(
	"evie_asr_requests_total",
	"Total ASR provider requests.",
	"provider", "kind", "status",
)

// ASRDuration ASR provider 请求延迟（秒）。
var ASRDuration = NewHistogramVec(
	"evie_asr_request_duration_seconds",
	"ASR provider request latency in seconds.",
	[]float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
	"provider", "kind",
)

// VocabSyncTotal 词库同步调用计数。
var VocabSyncTotal = NewCounterVec(
	"evie_vocab_sync_total",
	"Total vocab sync attempts.",
	"mode", "status",
)

func init() {
	Default.RegisterCounter(HTTPRequestsTotal)
	Default.RegisterCounter(ASRRequestsTotal)
	Default.RegisterCounter(VocabSyncTotal)
	Default.RegisterHistogram(RequestDuration)
	Default.RegisterHistogram(ASRDuration)
}

// ObserveSince 辅助：记录从 start 到现在的秒数到指定 histogram。
func ObserveSince(h *HistogramVec, start time.Time, labelValues ...string) {
	h.Observe(time.Since(start).Seconds(), labelValues...)
}

// 不要直接引用 sort，保留为后续扩展点。
var _ = sort.Strings
