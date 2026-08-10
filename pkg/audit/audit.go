// Package audit provides a protocol-independent operation audit abstraction.
// It has zero dependencies on proto-generated types or auth frameworks.
package audit

import "context"

// Record represents an operation audit entry, independent of proto types.
type Record struct {
	TenantID       uint32
	OperatorID     uint32
	OperatorName   string
	Module         string
	Action         string
	ResourceType   string
	ResourceID     string
	Method         string
	Path           string
	RequestSummary string
	IP             string
	UserAgent      string
	TraceID        string
	Success        bool
	DurationMs     int64
	ErrorMessage   string
}

// Client records operation audit entries.
type Client interface {
	Append(ctx context.Context, record *Record) error
}

// UserInfo carries the current request's user identity.
type UserInfo struct {
	TenantID   uint32
	UserID     uint32
	UserName   string
}

// ContextExtractor extracts UserInfo from a context. Callers inject
// their own implementation (typically based on authn).
type ContextExtractor func(ctx context.Context) UserInfo
