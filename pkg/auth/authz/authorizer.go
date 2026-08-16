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

// Tenant 租户类型，表示资源所属租户范围。
type Tenant string

// Policy 策略类型，表示授权策略
type Policy struct {
	// Subject 主体
	Subject Subject
	// Object 对象
	Object Object
	// Action 操作
	Action Action
	// Tenant 租户
	Tenant Tenant
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

// Authorizer 授权器接口：组合鉴权核心能力（Enforce）+ 策略管理 + RBAC 管理。
//
// 完整实现（如本地 Casbin）需实现全部能力；
// 委托型实现（如跨服务 gRPC 委托）只需实现 Enforcer，见 enforcer.go。
type Authorizer interface {
	Enforcer
	PolicyManager
	RoleManager

	// Init 初始化授权器
	Init(ctx context.Context, opts ...Option) error
	// Name 获取授权器名称
	Name() string
	// Options 返回当前配置选项
	Options() Options
	// Close 关闭授权器，释放资源
	Close() error
}

// AuthzProvider 授权提供者接口
type AuthzProvider interface {
	// Name 获取提供者名称
	Name() string
	// NewAuthorizer 创建新的授权器实例
	NewAuthorizer(ctx context.Context, opts ...Option) (Authorizer, error)
}
