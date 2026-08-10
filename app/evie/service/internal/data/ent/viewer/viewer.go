package viewer

import (
	"context"

	"backend-service/pkg/auth/authn"
)

type contextKey struct{}

// Viewer carries the data-scope identity used by Ent privacy policies.
type Viewer struct {
	tenantID uint32
	system   bool
}

// NewTenantContext returns a context scoped to one tenant.
func NewTenantContext(parent context.Context, tenantID uint32) context.Context {
	return context.WithValue(parent, contextKey{}, Viewer{tenantID: tenantID})
}

// NewSystemContext returns a context that bypasses tenant privacy filters.
func NewSystemContext(parent context.Context) context.Context {
	return context.WithValue(parent, contextKey{}, Viewer{system: true})
}

// FromContext returns the explicit Ent viewer from context.
func FromContext(ctx context.Context) (Viewer, bool) {
	v, ok := ctx.Value(contextKey{}).(Viewer)
	return v, ok
}

// TenantID resolves the tenant id from the explicit Ent viewer first, and then
// from the authenticated user carried by the HTTP/gRPC auth middleware.
func TenantID(ctx context.Context) (uint32, bool) {
	if v, ok := FromContext(ctx); ok {
		if v.tenantID > 0 {
			return v.tenantID, true
		}
		return 0, false
	}
	tenantID := authn.GetAuthUserTenantID(ctx)
	return tenantID, tenantID > 0
}

// IsSystem reports whether this context is allowed to bypass tenant filters.
func IsSystem(ctx context.Context) bool {
	v, ok := FromContext(ctx)
	return ok && v.system
}
