package authn

import (
	"time"
)

// Options 认证器配置选项
type Options struct {
	// Issuer 签发者标识
	Issuer string
	// Audience 目标接收者
	Audience []string
	// TokenExpiration 令牌过期时间
	TokenExpiration time.Duration
	// RefreshTokenExpiration 刷新令牌过期时间
	RefreshTokenExpiration time.Duration
	// SigningMethod 签名方法
	SigningMethod string
	// SigningKey 签名密钥
	SigningKey interface{}
	// VerificationKey 验证密钥
	VerificationKey interface{}
	// TokenLookup 令牌查找位置
	TokenLookup string
	// TokenHeadName 令牌头名称
	TokenHeadName string
	// ContextType 上下文类型
	ContextType ContextType
	// ClaimsFactory 声明工厂函数
	ClaimsFactory func() interface{}
	// UserFactory 用户工厂函数
	UserFactory func(*AuthClaims) SecurityUser
	// EnableRefresh 是否启用刷新
	EnableRefresh bool
	// EnableRevocation 是否启用撤销
	EnableRevocation bool
	// ProviderOptions 提供者特定选项
	ProviderOptions map[string]interface{}
}

// Option 选项函数类型
type Option func(*Options)

// DefaultOptions 默认选项
func DefaultOptions() Options {
	return Options{
		Issuer:                 "go-auth",
		Audience:               []string{"go-auth-api"},
		TokenExpiration:        time.Hour,
		RefreshTokenExpiration: time.Hour * 24 * 7,
		SigningMethod:          "HS256",
		TokenLookup:            "header:" + HeaderAuthorize,
		TokenHeadName:          BearerWord,
		ContextType:            ContextTypeKratosMetaData,
		EnableRefresh:          true,
		EnableRevocation:       false,
		ProviderOptions:        make(map[string]interface{}),
	}
}

// WithIssuer 设置签发者
func WithIssuer(issuer string) Option {
	return func(o *Options) {
		o.Issuer = issuer
	}
}

// WithAudience 设置接收者
func WithAudience(audience ...string) Option {
	return func(o *Options) {
		o.Audience = audience
	}
}

// WithTokenExpiration 设置令牌过期时间
func WithTokenExpiration(d time.Duration) Option {
	return func(o *Options) {
		o.TokenExpiration = d
	}
}

// WithRefreshTokenExpiration 设置刷新令牌过期时间
func WithRefreshTokenExpiration(d time.Duration) Option {
	return func(o *Options) {
		o.RefreshTokenExpiration = d
	}
}

// WithSigningMethod 设置签名方法
func WithSigningMethod(method string) Option {
	return func(o *Options) {
		o.SigningMethod = method
	}
}

// WithSigningKey 设置签名密钥
func WithSigningKey(key interface{}) Option {
	return func(o *Options) {
		o.SigningKey = key
	}
}

// WithVerificationKey 设置验证密钥
func WithVerificationKey(key interface{}) Option {
	return func(o *Options) {
		o.VerificationKey = key
	}
}

// WithTokenLookup 设置令牌查找位置
func WithTokenLookup(lookup string) Option {
	return func(o *Options) {
		o.TokenLookup = lookup
	}
}

// WithTokenHeadName 设置令牌头名称
func WithTokenHeadName(name string) Option {
	return func(o *Options) {
		o.TokenHeadName = name
	}
}

// WithContextType 设置上下文类型
func WithContextType(ct ContextType) Option {
	return func(o *Options) {
		o.ContextType = ct
	}
}

// WithClaimsFactory 设置声明工厂
func WithClaimsFactory(factory func() interface{}) Option {
	return func(o *Options) {
		o.ClaimsFactory = factory
	}
}

// WithUserFactory 设置用户工厂
func WithUserFactory(factory func(*AuthClaims) SecurityUser) Option {
	return func(o *Options) {
		o.UserFactory = factory
	}
}

// WithEnableRefresh 设置是否启用刷新
func WithEnableRefresh(enable bool) Option {
	return func(o *Options) {
		o.EnableRefresh = enable
	}
}

// WithEnableRevocation 设置是否启用撤销
func WithEnableRevocation(enable bool) Option {
	return func(o *Options) {
		o.EnableRevocation = enable
	}
}

// WithProviderOption 设置提供者特定选项
func WithProviderOption(key string, value interface{}) Option {
	return func(o *Options) {
		if o.ProviderOptions == nil {
			o.ProviderOptions = make(map[string]interface{})
		}
		o.ProviderOptions[key] = value
	}
}
