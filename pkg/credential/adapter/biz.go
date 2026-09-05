// Package adapter bridges credential.CallerIdentity with
// application-specific ctx helpers.
//
// # Purpose
//
// The credential package itself is business-system agnostic. To let a
// CallerIdentity travel through an existing application context that
// expects a particular auth interface (for example, biz.AuthContext
// in evie/tool), this package provides lightweight bridges.
//
// # Example
//
//	import (
//	    "backend-service/pkg/credential"
//	    "backend-service/pkg/credential/middleware"
//	    "backend-service/pkg/credential/adapter"
//	)
//
//	mw := middleware.HTTPMiddleware(middleware.Config{Provider: p})
//	// After mw runs, retrieve identity with either:
//	//   id, ok := middleware.FromContext(ctx)
//	//   id, ok := adapter.AuthFrom(ctx)
package adapter

import (
	"context"

	"backend-service/pkg/credential"
)

// CallerIdentity is an alias for credential.CallerIdentity so existing
// imports keep working.
type CallerIdentity = credential.CallerIdentity

// AuthContext is the minimal interface expected by callers that
// already use an "access token + tenant id" pattern.
type AuthContext interface {
	GetAccessToken() string
	GetTenantID() string
}

// ctxKey is unexported to avoid collisions.
type ctxKey struct{}

// WithAuth attaches id to ctx. Returns ctx unchanged if id is nil.
func WithAuth(ctx context.Context, id *CallerIdentity) context.Context {
	if id == nil {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, id)
}

// AuthFrom returns the CallerIdentity attached by WithAuth, or
// (nil, false) when none is present.
func AuthFrom(ctx context.Context) (*CallerIdentity, bool) {
	v, ok := ctx.Value(ctxKey{}).(*CallerIdentity)
	return v, ok
}

// CopyAuthContext copies the CallerIdentity from src into a fresh ctx
// detached from src (i.e. cancelling src does not affect dst). Used by
// background workers that need credentials but must outlive the
// originating request.
func CopyAuthContext(dst, src context.Context) context.Context {
	if id, ok := AuthFrom(src); ok {
		return WithAuth(dst, id)
	}
	return dst
}
