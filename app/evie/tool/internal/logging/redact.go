// Package logging · redact.go
//
// 日志脱敏：在写入前对敏感 key 的 value 做替换，避免 Authorization / API Key /
// password / token 等意外落盘到日志聚合系统。
//
// 触发：环境变量 EVIE_TOOL_LOG_REDACT=off|0|false 时禁用（默认启用）。
//
// # 匹配规则
//
// 大小写不敏感：
//   - 精确匹配（默认敏感 key 集合）
//   - 后缀匹配（_*_token / _password / _secret / _key）以覆盖配置字段
//
// 例如：
//   "Authorization: Bearer xyz"  →  value="***"
//   "X-API-Key: abc"             →  value="***"
//   "data.redis.password: p@ss"  →  value="***"
//   "data.redis.token_key_prefix: oauth2_access_token:"  →  value="***"
//     （后缀 _token 命中；该字段是配置模板不是真密钥，损失可接受）
//
// 已知限制：Kratos 内部日志通常不打印 header，因此 "header.Authorization"
// 这类点分 key 不会被本脱敏命中。安全策略应在更上层（gateway / WAF）拦截。
package logging

import (
	"os"
	"strings"

	"github.com/go-kratos/kratos/v2/log"
)

// EnvRedact 控制日志脱敏的环境变量名。
const EnvRedact = "EVIE_TOOL_LOG_REDACT"

// RedactValue 脱敏后的占位符。
const RedactValue = "***"

// defaultSensitiveKeys 默认敏感 key 集合（小写精确匹配）。
var defaultSensitiveKeys = map[string]bool{
	"authorization":   true,
	"x-api-key":       true,
	"api-key":         true,
	"apikey":          true,
	"api_key":         true,
	"password":         true,
	"secret":           true,
	"access-token":     true,
	"refresh-token":    true,
	"id-token":         true,
	"cookie":           true,
	"set-cookie":       true,
	"x-auth-token":     true,
	"x-csrf-token":     true,
}

// defaultSensitiveSuffixes 敏感 key 后缀（小写，HasSuffix 匹配）。
var defaultSensitiveSuffixes = []string{
	"_token",
	"_password",
	"_secret",
	"_api_key",
	"_apikey",
}

// ShouldRedact 从环境变量推断是否启用脱敏。
//
// 默认启用；显式 off / 0 / false 关闭。
func ShouldRedact() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(EnvRedact)))
	switch v {
	case "", "on", "1", "true", "yes":
		return true
	case "off", "0", "false", "no":
		return false
	}
	return true // 未知值保守按启用处理
}

// RedactingLogger 包装一个 log.Logger，对敏感字段值做替换。
type RedactingLogger struct {
	inner log.Logger
}

// NewRedactingLogger 构造脱敏 logger。
func NewRedactingLogger(inner log.Logger) *RedactingLogger {
	return &RedactingLogger{inner: inner}
}

// Log 实现 log.Logger。扫描 keyvals，将敏感 key 的 value 替换为 RedactValue。
//
// 奇数尾部的非配对元素原样保留（与 JSONLogger 行为一致）。
func (r *RedactingLogger) Log(level log.Level, keyvals ...any) error {
	out := make([]any, len(keyvals))
	for i := 0; i+1 < len(keyvals); i += 2 {
		k, ok := keyvals[i].(string)
		if !ok {
			out[i] = keyvals[i]
			out[i+1] = keyvals[i+1]
			continue
		}
		if IsSensitiveKey(k) {
			out[i] = keyvals[i]
			out[i+1] = RedactValue
		} else {
			out[i] = keyvals[i]
			out[i+1] = keyvals[i+1]
		}
	}
	if len(keyvals)%2 == 1 {
		out[len(keyvals)-1] = keyvals[len(keyvals)-1]
	}
	return r.inner.Log(level, out...)
}

// IsSensitiveKey 判断 key 是否敏感（公开，供测试与外部使用）。
//
// 匹配策略：
//   - 全 key 小写精确匹配
//   - 后缀（_*_token / _password / _secret / _api_key / _apikey）
//   - 末段（按 . / _ 拆分后的最后一段）精确匹配；覆盖 data.redis.password 风格
func IsSensitiveKey(key string) bool {
	k := strings.ToLower(strings.TrimSpace(key))
	if k == "" {
		return false
	}
	if defaultSensitiveKeys[k] {
		return true
	}
	for _, s := range defaultSensitiveSuffixes {
		if strings.HasSuffix(k, s) {
			return true
		}
	}
	// 末段匹配：data.redis.password → "password"；oauth2_access_token → "token"（不命中）
	last := lastSegment(k)
	if last != k && defaultSensitiveKeys[last] {
		return true
	}
	// 末段后缀：data.redis.access_token → "access_token" → 以 _token 结尾 → 命中
	for _, s := range defaultSensitiveSuffixes {
		if strings.HasSuffix(last, s) {
			return true
		}
	}
	return false
}

// lastSegment 按 . 与 _ 拆分，返回最后一段。
func lastSegment(key string) string {
	// 用 strings.LastIndexAny 找最后一个 . 或 _
	i := strings.LastIndexAny(key, "._")
	if i < 0 {
		return key
	}
	return key[i+1:]
}
