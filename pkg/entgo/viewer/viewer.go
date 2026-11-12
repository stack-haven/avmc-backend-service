// Package viewer 提供 Ent 隐私策略所需的「数据域身份」上下文。
//
// 它只依赖 pkg/auth/authn，不含任何服务符号，因此可被所有服务复用：
// 服务只需在自己的 data 层把租户身份注入 ctx，隐私规则统一从这里读取。
//
// 设计要点（见 docs/architecture 与 4-9 遗留项）：
//   - 显式 viewer 优先于认证中间件注入的用户租户；
//   - 缺少租户身份时返回 ok=false，由调用方（隐私规则）决定拒绝，
//     这里不做隐式放行，避免「忘记注入 = 看到全部」。
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
