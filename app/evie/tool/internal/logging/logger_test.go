package logging

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
)

func TestJSONLogger_OutputShape(t *testing.T) {
	var buf bytes.Buffer
	jl := NewJSONLogger(&buf)
	if err := jl.Log(log.LevelInfo, "msg", "server started", "service.id", "host-1", "port", 8080); err != nil {
		t.Fatalf("Log: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got); err != nil {
		t.Fatalf("not JSON: %v\nraw=%s", err, buf.String())
	}
	if got["level"] != "info" {
		t.Errorf("level=%v", got["level"])
	}
	if got["msg"] != "server started" {
		t.Errorf("msg=%v", got["msg"])
	}
	if got["service.id"] != "host-1" {
		t.Errorf("service.id=%v", got["service.id"])
	}
	if got["port"] != float64(8080) { // JSON numbers decode to float64
		t.Errorf("port=%v", got["port"])
	}
	if _, ok := got["ts"].(string); !ok {
		t.Errorf("ts type=%T", got["ts"])
	}
}

func TestJSONLogger_OddFieldsIgnored(t *testing.T) {
	var buf bytes.Buffer
	jl := NewJSONLogger(&buf)
	_ = jl.Log(log.LevelWarn, "msg", "odd", "k1", "v1", "stray") // 奇数尾部 → 忽略
	out := buf.String()
	if !strings.Contains(out, `"k1":"v1"`) {
		t.Errorf("missing k1: %s", out)
	}
	if !strings.Contains(out, `"msg":"odd"`) {
		t.Errorf("missing msg: %s", out)
	}
}

func TestJSONLogger_NonStringKeyIgnored(t *testing.T) {
	var buf bytes.Buffer
	jl := NewJSONLogger(&buf)
	// pairs: (123, "v") → 非 string 键 → skip; ("msg", "ok") → rec[msg]=ok; ("v2") → 奇数尾部 skip
	_ = jl.Log(log.LevelError, 123, "v", "msg", "ok", "v2")
	out := buf.String()
	if !strings.Contains(out, `"msg":"ok"`) {
		t.Errorf("expected msg:ok in: %s", out)
	}
	if strings.Contains(out, `"v2"`) {
		t.Errorf("stray v2 should be ignored: %s", out)
	}
}

func TestJSONLogger_ConcurrentSafe(t *testing.T) {
	var buf bytes.Buffer
	jl := NewJSONLogger(&buf)
	const N = 200
	done := make(chan struct{}, N)
	for i := 0; i < N; i++ {
		go func(i int) {
			_ = jl.Log(log.LevelInfo, "msg", "m", "i", i)
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < N; i++ {
		<-done
	}
	// 每行必须可单独解析（说明没有交错写）
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != N {
		t.Fatalf("lines=%d want=%d", len(lines), N)
	}
	for i, line := range lines {
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("line %d not JSON: %v\n%s", i, err, line)
		}
	}
}

func TestDetectFormat(t *testing.T) {
	cases := []struct {
		env  string
		want string
	}{
		{"", FormatText},
		{"json", FormatJSON},
		{"text", FormatText},
		{"JSON", "JSON"}, // 大小写敏感：仅识别小写
	}
	for _, c := range cases {
		t.Run(c.env, func(t *testing.T) {
			if c.env == "" {
				t.Setenv(EnvKey, "")
			} else {
				t.Setenv(EnvKey, c.env)
			}
			got := DetectFormat()
			if got != c.want {
				t.Errorf("DetectFormat()=%q want=%q", got, c.want)
			}
		})
	}
}

func TestNew_Switch(t *testing.T) {
	t.Setenv(EnvKey, "json")
	if got := New(DetectFormat()); got == nil {
		t.Error("nil logger")
	}
	t.Setenv(EnvKey, "text")
	if got := New(DetectFormat()); got == nil {
		t.Error("nil logger")
	}
}

func TestJSONLogger_AllLevels(t *testing.T) {
	cases := []struct {
		lvl  log.Level
		want string
	}{
		{log.LevelDebug, "debug"},
		{log.LevelInfo, "info"},
		{log.LevelWarn, "warn"},
		{log.LevelError, "error"},
	}
	for _, c := range cases {
		var buf bytes.Buffer
		_ = NewJSONLogger(&buf).Log(c.lvl, "msg", "x")
		var got map[string]any
		_ = json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got)
		if got["level"] != c.want {
			t.Errorf("level=%v want=%v", got["level"], c.want)
		}
	}
}
