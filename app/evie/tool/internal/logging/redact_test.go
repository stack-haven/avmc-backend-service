package logging

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
)

// captureLogger 返回一个把日志写到 buf 的 JSON logger（用于断言脱敏结果）。
func captureLogger() (*JSONLogger, *bytes.Buffer) {
	var buf bytes.Buffer
	return NewJSONLogger(&buf), &buf
}

func parseLine(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	line := strings.TrimRight(buf.String(), "\n")
	if line == "" {
		t.Fatal("no output")
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, line)
	}
	return m
}

func TestRedactingLogger_RedactsAuthorization(t *testing.T) {
	jl, buf := captureLogger()
	rl := NewRedactingLogger(jl)

	if err := rl.Log(log.LevelInfo, "msg", "auth", "Authorization", "Bearer secret-token"); err != nil {
		t.Fatalf("Log: %v", err)
	}
	m := parseLine(t, buf)
	if m["Authorization"] != "***" {
		t.Errorf("Authorization=%v, want ***", m["Authorization"])
	}
	if m["msg"] != "auth" {
		t.Errorf("msg=%v", m["msg"])
	}
}

func TestRedactingLogger_CaseInsensitive(t *testing.T) {
	cases := []string{"authorization", "AUTHORIZATION", "Authorization", "x-Api-Key", "X-API-KEY"}
	for _, k := range cases {
		t.Run(k, func(t *testing.T) {
			jl, buf := captureLogger()
			rl := NewRedactingLogger(jl)
			_ = rl.Log(log.LevelInfo, k, "value-should-not-leak")
			m := parseLine(t, buf)
			if m[k] != "***" {
				t.Errorf("key=%s value=%v, want ***", k, m[k])
			}
		})
	}
}

func TestRedactingLogger_RedactsBySuffix(t *testing.T) {
	cases := []struct {
		key string
	}{
		{"data.redis.password"},
		{"oauth2_access_token"},
		{"client_secret"},
		{"third_party_api_key"},
		{"user_apikey"},
	}
	for _, c := range cases {
		t.Run(c.key, func(t *testing.T) {
			jl, buf := captureLogger()
			rl := NewRedactingLogger(jl)
			_ = rl.Log(log.LevelInfo, c.key, "leaked-value")
			m := parseLine(t, buf)
			if m[c.key] != "***" {
				t.Errorf("key=%s value=%v, want ***", c.key, m[c.key])
			}
		})
	}
}

func TestRedactingLogger_KeepsNonSensitive(t *testing.T) {
	jl, buf := captureLogger()
	rl := NewRedactingLogger(jl)
	_ = rl.Log(log.LevelInfo,
		"msg", "normal",
		"service.name", "evie-tool",
		"http.method", "GET",
		"http.path", "/evie/tool/v1/enhance",
		"latency_ms", 12,
	)
	m := parseLine(t, buf)
	if m["service.name"] != "evie-tool" {
		t.Errorf("service.name leaked? got %v", m["service.name"])
	}
	if m["http.path"] != "/evie/tool/v1/enhance" {
		t.Errorf("http.path=%v", m["http.path"])
	}
	if m["latency_ms"] != float64(12) {
		t.Errorf("latency_ms=%v", m["latency_ms"])
	}
}

func TestRedactingLogger_OddFieldsPassThrough(t *testing.T) {
	jl, buf := captureLogger()
	rl := NewRedactingLogger(jl)
	_ = rl.Log(log.LevelWarn, "msg", "x", "Authorization", "secret", "stray")
	m := parseLine(t, buf)
	if m["Authorization"] != "***" {
		t.Errorf("Authorization=%v", m["Authorization"])
	}
}

func TestRedactingLogger_NonStringKey(t *testing.T) {
	jl, buf := captureLogger()
	rl := NewRedactingLogger(jl)
	// 非 string key：脱敏器原样转发；JSON 后端会丢弃非 string key（JSON 仅支持 string key）。
	_ = rl.Log(log.LevelError, 123, "some-value", "msg", "x")
	m := parseLine(t, buf)
	// msg 应该正常出现；非 string key 不应让脱敏器 panic 或篡改后续 string key。
	if m["msg"] != "x" {
		t.Errorf("msg=%v, want x", m["msg"])
	}
}

func TestShouldRedact_Defaults(t *testing.T) {
	cases := []struct {
		env  string
		want bool
	}{
		{"", true},
		{"on", true},
		{"1", true},
		{"true", true},
		{"yes", true},
		{"off", false},
		{"0", false},
		{"false", false},
		{"no", false},
		{"unknown-value", true}, // 未知值保守按启用
	}
	for _, c := range cases {
		t.Run(c.env, func(t *testing.T) {
			if c.env == "" {
				t.Setenv(EnvRedact, "")
			} else {
				t.Setenv(EnvRedact, c.env)
			}
			if got := ShouldRedact(); got != c.want {
				t.Errorf("ShouldRedact()=%v want=%v", got, c.want)
			}
		})
	}
}

func TestIsSensitiveKey(t *testing.T) {
	cases := []struct {
		key  string
		want bool
	}{
		{"Authorization", true},
		{"authorization", true},
		{"X-API-Key", true},
		{"x-api-key", true},
		{"password", true},
		{"data.redis.password", true},
		{"oauth2_access_token", true},
		{"client_secret", true},
		{"service.name", false},
		{"http.path", false},
		{"latency_ms", false},
		{"tenant_id", false},
		{"", false},
	}
	for _, c := range cases {
		t.Run(c.key, func(t *testing.T) {
			if got := IsSensitiveKey(c.key); got != c.want {
				t.Errorf("IsSensitiveKey(%q)=%v want=%v", c.key, got, c.want)
			}
		})
	}
}

func TestRedactingLogger_PreservesInnerInterface(t *testing.T) {
	// 验证 RedactingLogger 仍实现 log.Logger
	var _ log.Logger = (*RedactingLogger)(nil)
}

func TestRedactValueConst(t *testing.T) {
	if RedactValue != "***" {
		t.Errorf("RedactValue=%q", RedactValue)
	}
}
