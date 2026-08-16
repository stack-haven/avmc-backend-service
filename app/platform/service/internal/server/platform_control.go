package server

import (
	"context"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"

	"backend-service/app/platform/service/internal/authzpolicy"
	"backend-service/pkg/auth/authn"
)

var (
	// ErrPlatformOperatorRequired is returned when a non-platform-operator
	// attempts to access a platform control-plane operation.
	ErrPlatformOperatorRequired = errors.Forbidden(
		"PLATFORM_OPERATOR_REQUIRED",
		"platform operator privileges required for this operation",
	)
)

// platformControlServer returns a middleware that enforces platform operator
// privileges for all platform control-plane operations. Operations that are
// not classified as platform control-plane (tenant data-plane ops) pass
// through without additional checks.
//
// This implements the fourth layer of the defense-in-depth model:
//  1. JWT signature verification (gateway)
//  2. Casbin RBAC enforcement (authz middleware)
//  3. Ent Privacy tenant isolation (data layer)
//  4. Platform operator check (this middleware)
func platformControlServer() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			// Resolve the operation name for this request.
			tr, ok := transport.FromServerContext(ctx)
			if !ok {
				// No transport context — defer to handler (should not happen in practice).
				return handler(ctx, req)
			}
			operation := tr.Operation()

			// Only enforce platform operator for platform control-plane ops.
			if !authzpolicy.IsPlatformControlOperation(operation) {
				return handler(ctx, req)
			}

			// Verify the authenticated user is a platform operator.
			if !authn.IsPlatformOperator(ctx) {
				return nil, ErrPlatformOperatorRequired
			}

			return handler(ctx, req)
		}
	}
}
