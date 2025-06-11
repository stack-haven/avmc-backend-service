package authz

import (
	"context"
)

// Subject 主体类型，表示执行操作的实体（通常是用户）
type Subject string

// Object 对象类型，表示被操作的资源
type Object string

// Action 操作类型，表示对资源执行的操作
type Action string

// Domain 域类型，表示资源所属的域（如租户、项目等）
type Domain string

// Policy 策略类型，表示授权策略
type Policy struct {
	// Subject 主体
	Subject Subject
	// Object 对象
	Object Object
	// Action 操作
	Action Action
	// Domain 域
	Domain Domain
	// Effect 效果（允许或拒绝）
	Effect Effect
}

// Effect 效果类型，表示策略的效果
type Effect string

// 效果常量
const (
	// EffectAllow 允许
	EffectAllow Effect = "allow"
	// EffectDeny 拒绝
	EffectDeny Effect = "deny"
)

// Authorizer 授权器接口
type Authorizer interface {
	// Init 初始化授权器
	Init(ctx context.Context, opts ...Option) error

	// Enforce 执行授权检查
	// ctx: 上下文信息
	// sub: 主体
	// obj: 对象
	// act: 操作
	// domain: 域
	// 返回: 是否授权和可能的错误
	Enforce(ctx context.Context, sub Subject, obj Object, act Action, domain Domain) (bool, error)

	// BatchEnforce 批量执行授权检查
	// ctx: 上下文信息
	// subjects: 主体列表
	// objects: 对象列表
	// actions: 操作列表
	// domains: 域列表
	// 返回: 授权结果列表和可能的错误
	BatchEnforce(ctx context.Context, subjects []Subject, objects []Object, actions []Action, domains []Domain) ([]bool, error)

	// AddPolicy 添加策略
	// ctx: 上下文信息
	// policy: 策略
	// 返回: 是否成功添加和可能的错误
	AddPolicy(ctx context.Context, policy Policy) (bool, error)

	// RemovePolicy 移除策略
	// ctx: 上下文信息
	// policy: 策略
	// 返回: 是否成功移除和可能的错误
	RemovePolicy(ctx context.Context, policy Policy) (bool, error)

	// AddPolicies 批量添加策略
	// ctx: 上下文信息
	// policies: 策略列表
	// 返回: 是否成功添加和可能的错误
	AddPolicies(ctx context.Context, policies []Policy) (bool, error)

	// RemovePolicies 批量移除策略
	// ctx: 上下文信息
	// policies: 策略列表
	// 返回: 是否成功移除和可能的错误
	RemovePolicies(ctx context.Context, policies []Policy) (bool, error)

	// GetAllSubjects 获取所有主体
	// ctx: 上下文信息
	// 返回: 主体列表和可能的错误
	GetAllSubjects(ctx context.Context) ([]Subject, error)

	// GetAllObjects 获取所有对象
	// ctx: 上下文信息
	// 返回: 对象列表和可能的错误
	GetAllObjects(ctx context.Context) ([]Object, error)

	// GetAllActions 获取所有操作
	// ctx: 上下文信息
	// 返回: 操作列表和可能的错误
	GetAllActions(ctx context.Context) ([]Action, error)

	// GetAllDomains 获取所有域
	// ctx: 上下文信息
	// 返回: 域列表和可能的错误
	GetAllDomains(ctx context.Context) ([]Domain, error)

	// GetAllRoles 获取所有角色
	// ctx: 上下文信息
	// 返回: 角色列表和可能的错误
	GetAllRoles(ctx context.Context) ([]Subject, error)

	// GetRolesForUser 获取用户的角色
	// ctx: 上下文信息
	// user: 用户
	// domain: 域
	// 返回: 角色列表和可能的错误
	GetRolesForUser(ctx context.Context, user Subject, domain Domain) ([]Subject, error)

	// GetUsersForRole 获取角色的用户
	// ctx: 上下文信息
	// role: 角色
	// domain: 域
	// 返回: 用户列表和可能的错误
	GetUsersForRole(ctx context.Context, role Subject, domain Domain) ([]Subject, error)

	// HasRoleForUser 检查用户是否拥有角色
	// ctx: 上下文信息
	// user: 用户
	// role: 角色
	// domain: 域
	// 返回: 是否拥有角色和可能的错误
	HasRoleForUser(ctx context.Context, user Subject, role Subject, domain Domain) (bool, error)

	// AddRoleForUser 为用户添加角色
	// ctx: 上下文信息
	// user: 用户
	// role: 角色
	// domain: 域
	// 返回: 是否成功添加和可能的错误
	AddRoleForUser(ctx context.Context, user Subject, role Subject, domain Domain) (bool, error)

	// DeleteRoleForUser 删除用户的角色
	// ctx: 上下文信息
	// user: 用户
	// role: 角色
	// domain: 域
	// 返回: 是否成功删除和可能的错误
	DeleteRoleForUser(ctx context.Context, user Subject, role Subject, domain Domain) (bool, error)

	// Name 获取授权器名称
	// 返回: 授权器名称
	Name() string

	// 返回: 选项允许您查看当前选项。
	Options() Options

	// Close 关闭授权器，释放资源
	// 返回: 可能的错误
	Close() error
}

// AuthzProvider 授权提供者接口
type AuthzProvider interface {
	// Name 获取提供者名称
	// 返回: 提供者名称
	Name() string

	// NewAuthorizer 创建新的授权器实例
	// ctx: 上下文信息
	// opts: 配置选项
	// 返回: 授权器实例和可能的错误
	NewAuthorizer(ctx context.Context, opts ...Option) (Authorizer, error)
}
