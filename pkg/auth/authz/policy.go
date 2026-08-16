package authz

import "context"

// PolicyManager 定义策略管理接口：增删策略、查询策略元素。
//
// 只有本地维护策略的鉴权引擎（如 Casbin）需要实现；
// 委托型实现（gRPC 委托）无需实现本接口。
type PolicyManager interface {
	AddPolicy(ctx context.Context, policy Policy) (bool, error)
	RemovePolicy(ctx context.Context, policy Policy) (bool, error)
	AddPolicies(ctx context.Context, policies []Policy) (bool, error)
	RemovePolicies(ctx context.Context, policies []Policy) (bool, error)
	GetAllSubjects(ctx context.Context) ([]Subject, error)
	GetAllObjects(ctx context.Context) ([]Object, error)
	GetAllActions(ctx context.Context) ([]Action, error)
	GetAllTenants(ctx context.Context) ([]Tenant, error)
	GetAllRoles(ctx context.Context) ([]Subject, error)
}
