package authz

import "context"

// RoleManager 定义角色关系管理接口：用户-角色的 RBAC 绑定。
//
// 只有本地维护角色关系的鉴权引擎需要实现；
// 委托型实现无需实现本接口。
type RoleManager interface {
	GetRolesForUser(ctx context.Context, user Subject, tenant Tenant) ([]Subject, error)
	GetUsersForRole(ctx context.Context, role Subject, tenant Tenant) ([]Subject, error)
	HasRoleForUser(ctx context.Context, user Subject, role Subject, tenant Tenant) (bool, error)
	AddRoleForUser(ctx context.Context, user Subject, role Subject, tenant Tenant) (bool, error)
	DeleteRoleForUser(ctx context.Context, user Subject, role Subject, tenant Tenant) (bool, error)
}
