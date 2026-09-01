package id

import "github.com/google/uuid"

// SessionID 业务前缀（区分不同业务来源）。
const (
	SessionIDPrefixASR            = "asr-"
	SessionIDPrefixReRecognize     = "re-"
	SessionIDPrefixEnhanceText     = "ext-"
	SessionIDPrefixEnhancePipeline = "pipe-"
)

// NewSessionID 生成统一格式的 session_id。
// 格式: "{prefix}{uuid-v4}"
// 例: "asr-550e8400-e29b-41d4-a716-446655440000"
//
// 使用 google/uuid v4 (random based) 保证唯一性；业务前缀便于按来源过滤日志。
func NewSessionID(prefix string) string {
	return prefix + uuid.NewString()
}
