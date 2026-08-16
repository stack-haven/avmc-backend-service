package authz

import "context"

// Enforcer 定义鉴权核心接口：执行权限判断。
//
// 这是所有鉴权实现的最小契约。纯委托型实现（如跨服务 gRPC 委托）
// 只需实现 Enforcer，无需背负策略/RBAC 管理的空实现。
type Enforcer interface {
	// Enforce 执行授权检查。
	// sub: 主体（用户），obj: 对象（资源），act: 操作，tenant: 租户。
	// 返回是否允许与可能的错误。
	Enforce(ctx context.Context, sub Subject, obj Object, act Action, tenant Tenant) (bool, error)

	// BatchEnforce 批量执行授权检查，结果与输入一一对应。
	BatchEnforce(ctx context.Context, subs []Subject, objs []Object, acts []Action, tenants []Tenant) ([]bool, error)
}
