// Package processors · status.go
// Status 枚举（per-request 增强结果状态）。
//
// HA 语义：
//   - StatusSuccess：全部 processors 正常
//   - StatusPartial：部分 processors 失败（errors 非空 + pipeline 未 panic）
//   - StatusCanceled：ctx 取消
//   - StatusFailed：致命错误（如 vocab 完全缺失 + 必需 processor）
//   - StatusPanic：pipeline 内部 panic（已 recover）
package processors

// Status 状态码。
const (
	StatusUnknown  int32 = 0
	StatusSuccess  int32 = 1
	StatusPartial  int32 = 2
	StatusCanceled int32 = 3
	StatusFailed   int32 = 4
	StatusPanic    int32 = 5
)

// StatusName 返回状态码可读名（用于日志 / 调试）。
func StatusName(s int32) string {
	switch s {
	case StatusSuccess:
		return "SUCCESS"
	case StatusPartial:
		return "PARTIAL"
	case StatusCanceled:
		return "CANCELED"
	case StatusFailed:
		return "FAILED"
	case StatusPanic:
		return "PANIC"
	default:
		return "UNKNOWN"
	}
}