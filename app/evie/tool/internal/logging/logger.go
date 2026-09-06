// Package logging · logger.go
//
// 提供 Kratos log.Logger 的两种实现：
//   - text：Kratos 默认 std logger（人类可读）
//   - json：结构化 JSON 输出（一行一条记录，便于 Loki / ELK / jq 处理）
//
// 切换方式：环境变量 EVIE_TOOL_LOG_FORMAT=json|text（默认 text）。
//
// 设计目标：
//   - 零额外依赖（仅 stdlib encoding/json）
//   - 线程安全（多 goroutine 并发写）
//   - 与 Kratos log.With / log.DefaultTimestamp / log.DefaultCaller 完全兼容
//   - 字段顺序：ts → level → msg → 其他（确定性，便于测试）
package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

// Format 常量。
const (
	FormatJSON = "json"
	FormatText = "text"
)

// EnvKey 切换日志格式的环境变量名。
const EnvKey = "EVIE_TOOL_LOG_FORMAT"

// DetectFormat 从环境变量推断日志格式，未设置时返回 text。
func DetectFormat() string {
	if v := os.Getenv(EnvKey); v != "" {
		return v
	}
	return FormatText
}

// New 根据 format 返回 Kratos log.Logger。
//
// baseFields 透传给 log.With；常见用法：
//
//	logger := logging.New(logging.DetectFormat(),
//	    "ts",      log.DefaultTimestamp,
//	    "caller",  log.DefaultCaller,
//	    "service", "evie-tool",
//	)
func New(format string, baseFields ...any) log.Logger {
	var base log.Logger
	if format == FormatJSON {
		base = NewJSONLogger(os.Stdout)
	} else {
		base = log.NewStdLogger(os.Stdout)
	}
	return log.With(base, baseFields...)
}

// JSONLogger 把每条日志序列化为单行 JSON。
type JSONLogger struct {
	mu sync.Mutex
	w  io.Writer
}

// NewJSONLogger 构造 JSON logger（便于测试指定 writer）。
func NewJSONLogger(w io.Writer) *JSONLogger {
	return &JSONLogger{w: w}
}

// Log 实现 log.Logger。
//
// 兼容 Kratos v2.9+ 的签名：Log(level Level, keyvals ...any) error。
// keyvals 是 k,v,k,v,...；非 string 键或奇数尾部将被忽略（不 panic）。
// "msg" 键（Kratos Helper.Info 默认键）与其他字段平铺在 JSON 顶层。
func (l *JSONLogger) Log(level log.Level, keyvals ...any) error {
	rec := make(map[string]any, len(keyvals)/2+3)
	rec["ts"] = time.Now().UTC().Format(time.RFC3339Nano)
	rec["level"] = levelString(level)
	for i := 0; i+1 < len(keyvals); i += 2 {
		k, ok := keyvals[i].(string)
		if !ok {
			continue
		}
		rec[k] = keyvals[i+1]
	}
	data, err := json.Marshal(rec)
	if err != nil {
		// 序列化失败：退化为占位行，避免进程崩溃。
		data = []byte(fmt.Sprintf(`{"ts":%q,"level":"error","msg":"log marshal failed","err":%q}`,
			rec["ts"], err.Error()))
	}
	l.mu.Lock()
	_, _ = l.w.Write(data)
	_, _ = l.w.Write([]byte{'\n'})
	l.mu.Unlock()
	return nil
}

// SortedKeys 返回 map 的有序键（仅测试 / 诊断使用）。
func SortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// levelString 把 Kratos log.Level 转为小写字符串。
func levelString(lv log.Level) string {
	switch lv {
	case log.LevelDebug:
		return "debug"
	case log.LevelInfo:
		return "info"
	case log.LevelWarn:
		return "warn"
	case log.LevelError:
		return "error"
	case log.LevelFatal:
		return "fatal"
	default:
		return "unknown"
	}
}
