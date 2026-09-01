package biz

import "backend-service/pkg/utils/id"

// SessionID 基础逻辑：调用 id 包生成业务前缀的 session_id。
// 作为 biz 层基础逻辑的归属，所有 service 入口都从这里拿 ID。

// NewASRSessionID 语音识别请求的 session_id（ASR 入口）。
func NewASRSessionID() string { return id.NewSessionID(id.SessionIDPrefixASR) }

// NewEnhanceTextSessionID 纯文本增强请求的 session_id（EnhanceText 入口）。
func NewEnhanceTextSessionID() string {
	return id.NewSessionID(id.SessionIDPrefixEnhanceText)
}

// NewPipelineSessionID ASR 内部自动增强的 session_id（ASR→Enhance 共享同一 ID）。
func NewPipelineSessionID() string {
	return id.NewSessionID(id.SessionIDPrefixEnhancePipeline)
}
