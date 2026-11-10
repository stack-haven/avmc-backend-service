package viewer

import (
	"context"

	"backend-service/pkg/auth/authn"
)

type contextKey struct{}

// Viewer 携带 Ent 隐私策略使用的数据域身份。
type Viewer struct {
	tenantID uint32
	system   bool
}

// NewTenantContext 返回绑定单个租户的上下文。
func NewTenantContext(parent context.Context, tenantID uint32) context.Context {
	return context.WithValue(parent, contextKey{}, Viewer{tenantID: tenantID})
}

// NewSystemContext 返回跳过租户隐私过滤的上下文（仅迁移、种子等系统场景使用）。
func NewSystemContext(parent context.Context) context.Context {
	return context.WithValue(parent, contextKey{}, Viewer{system: true})
}

// FromContext 返回显式设置的 Ent viewer。
func FromContext(ctx context.Context) (Viewer, bool) {
	v, ok := ctx.Value(contextKey{}).(Viewer)
	return v, ok
}

// TenantID 优先取显式 viewer，其次取认证中间件注入的用户租户。
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

// IsSystem 报告当前上下文是否允许绕过租户过滤。
func IsSystem(ctx context.Context) bool {
	v, ok := FromContext(ctx)
	return ok && v.system
}
