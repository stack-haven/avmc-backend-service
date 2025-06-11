package authn

import (
	"context"
	"time"
)

// Authenticator 定义了身份验证服务的接口
// 实现此接口的提供者需要支持身份验证、令牌管理等功能
type Authenticator interface {
	// Init 初始化认证器
	// ctx: 上下文信息
	// 返回: 初始化过程中的错误
	Init(ctx context.Context, opts ...Option) error

	// Authenticate 验证用户身份并返回认证声明
	// ctx: 上下文信息，可能包含令牌信息
	// 返回: 认证声明和可能的错误
	Authenticate(ctx context.Context) (*AuthClaims, error)

	// ValidateToken 验证令牌的有效性
	// ctx: 上下文信息
	// token: 待验证的令牌字符串
	// 返回: 认证声明和可能的错误
	ValidateToken(ctx context.Context, token string) (*AuthClaims, error)

	// CreateToken 创建新的身份令牌
	// ctx: 上下文信息
	// claims: 认证声明信息
	// 返回: 令牌字符串和可能的错误
	CreateToken(ctx context.Context, claims AuthClaims) (string, error)

	// RefreshToken 刷新令牌，延长有效期
	// ctx: 上下文信息
	// token: 待刷新的令牌字符串
	// 返回: 新的令牌字符串和可能的错误
	RefreshToken(ctx context.Context, token string) (string, error)

	// RevokeToken 撤销令牌，使其失效
	// ctx: 上下文信息
	// token: 待撤销的令牌字符串
	// 返回: 可能的错误
	RevokeToken(ctx context.Context, token string) error

	// Close 关闭认证器，释放资源
	// 返回: 可能的错误
	Close() error

	// 返回: 选项允许您查看当前选项。
	Options() Options

	// Name 返回认证提供者的名称
	// 返回: 提供者名称
	Name() string
}

// SecurityUser 定义了安全用户信息的接口
type SecurityUser interface {
	// Name 获取Security Name
	// 返回: Security Name标识字符串
	Name() string
	// ParseFromContext 从上下文中解析用户信息
	// ctx: 上下文信息
	// 返回: 可能的错误
	ParseFromContext(ctx context.Context) error

	// GetSubject 获取主体标识（通常是用户ID）
	// 返回: 主体标识字符串
	GetSubject() string

	// GetObject 获取对象标识（通常是资源ID）
	// 返回: 对象标识字符串
	GetObject() string

	// GetAction 获取操作标识
	// 返回: 操作标识字符串
	GetAction() string

	// GetDomain 获取域标识（通常是租户或项目ID）
	// 返回: 域标识字符串
	GetDomain() string
}

// SecurityUserCreator 定义了从认证声明创建安全用户的函数类型
type SecurityUserCreator func(*AuthClaims) SecurityUser

// TokenManager 定义了令牌管理的接口
// 实现此接口的提供者需要支持令牌的创建、验证、刷新和撤销功能
type TokenManager interface {
	// CreateToken 创建新的身份令牌
	// ctx: 上下文信息
	// claims: 认证声明信息
	// expiration: 令牌有效期
	// 返回: 令牌字符串和可能的错误
	CreateToken(ctx context.Context, claims AuthClaims, expiration time.Duration) (string, error)

	// ValidateToken 验证令牌的有效性
	// ctx: 上下文信息
	// token: 待验证的令牌字符串
	// 返回: 认证声明和可能的错误
	ValidateToken(ctx context.Context, token string) (*AuthClaims, error)

	// RefreshToken 刷新令牌，延长有效期
	// ctx: 上下文信息
	// token: 待刷新的令牌字符串
	// 返回: 新的令牌字符串和可能的错误
	RefreshToken(ctx context.Context, token string) (string, error)

	// RevokeToken 撤销令牌，使其失效
	// ctx: 上下文信息
	// token: 待撤销的令牌字符串
	// 返回: 可能的错误
	RevokeToken(ctx context.Context, token string) error
}

// AuthProvider 定义了认证提供者的接口
// 实现此接口的提供者需要支持创建认证器实例
type AuthProvider interface {
	// Name 获取提供者名称
	// 返回: 提供者名称
	Name() string
	// NewAuthenticator 创建新的认证器实例
	// ctx: 上下文信息
	// opts: 配置选项
	// 返回: 认证器实例和可能的错误
	NewAuthenticator(ctx context.Context, opts ...Option) (Authenticator, error)
}
