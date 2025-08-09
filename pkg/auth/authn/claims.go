package authn

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/transport"
)

// 上下文键类型
type ctxKey string

// 上下文类型枚举
type ContextType int

// 常量定义
const (
	// HeaderAuthorize 认证头名称
	HeaderAuthorize = "Authorization"
	// BearerWord Bearer认证方案前缀
	BearerWord = "Bearer"
	// BasicWord Basic认证方案前缀
	BasicWord = "Basic"
)

// 上下文类型常量
const (
	// ContextTypeGrpc gRPC上下文类型
	ContextTypeGrpc ContextType = iota
	// ContextTypeKratosMetaData Kratos元数据上下文类型
	ContextTypeKratosMetaData
	// ContextTypeHTTP HTTP上下文类型
	ContextTypeHTTP
)

// 上下文键常量
var (
	// authClaimsContextKey 认证声明上下文键
	authClaimsContextKey = ctxKey("authn-claims")
	// authUserContextKey 认证用户上下文键
	authUserContextKey = ctxKey("authn-user")
)

// AuthClaims 认证声明类型，包含身份验证相关的信息
type AuthClaims map[string]interface{}

// StandardClaims 标准声明字段
type StandardClaims struct {
	// Subject 主体（通常是用户ID）
	Subject string `json:"sub,omitempty"`
	// Issuer 签发者
	Issuer string `json:"iss,omitempty"`
	// Audience 接收者
	Audience []string `json:"aud,omitempty"`
	// ExpiresAt 过期时间
	ExpiresAt *time.Time `json:"exp,omitempty"`
	// NotBefore 生效时间
	NotBefore *time.Time `json:"nbf,omitempty"`
	// IssuedAt 签发时间
	IssuedAt *time.Time `json:"iat,omitempty"`
	// ID 唯一标识符
	ID string `json:"jti,omitempty"`
	// Scope 权限范围
	// Scope []string `json:"scope,omitempty"`
}

// ParseContextTokenFunc 定义从上下文解析令牌的函数类型
type ParseContextTokenFunc func(context.Context) (string, error)

// ParseContextToken 从上下文中解析令牌
// headerKey: 头部键名
// scheme: 认证方案（如Bearer）
// 返回: 解析令牌的函数
func ParseContextToken(headerKey string, scheme string) ParseContextTokenFunc {
	return func(ctx context.Context) (string, error) {
		if header, ok := transport.FromServerContext(ctx); ok {
			tokenStr := header.RequestHeader().Get(headerKey)
			if tokenStr == "" {
				return "", ErrMissingToken
			}
			splits := strings.SplitN(tokenStr, " ", 2)
			if len(splits) < 2 {
				return "", ErrInvalidTokenFormat
			}

			if !strings.EqualFold(splits[0], scheme) {
				return "", ErrUnsupportedTokenScheme
			}
			return splits[1], nil
		}
		return "", ErrNoTransportContext
	}
}

// ContextWithAuthClaims 将认证声明注入上下文
// parent: 父上下文
// claims: 认证声明
// 返回: 新的上下文
func ContextWithAuthClaims(parent context.Context, claims *AuthClaims) context.Context {
	return context.WithValue(parent, authClaimsContextKey, claims)
}

// AuthClaimsFromContext 从上下文中提取认证声明
// ctx: 上下文
// 返回: 认证声明和是否存在的标志
func AuthClaimsFromContext(ctx context.Context) (*AuthClaims, bool) {
	claims, ok := ctx.Value(authClaimsContextKey).(*AuthClaims)
	if !ok {
		return nil, false
	}
	return claims, true
}

// ContextWithAuthUser 将认证用户注入上下文
// parent: 父上下文
// user: 认证用户
// 返回: 新的上下文
func ContextWithAuthUser(parent context.Context, user SecurityUser) context.Context {
	return context.WithValue(parent, authUserContextKey, user)
}

// AuthUserFromContext 从上下文中提取认证用户
// ctx: 上下文
// 返回: 认证用户和是否存在的标志
func AuthUserFromContext(ctx context.Context) (SecurityUser, bool) {
	user, ok := ctx.Value(authUserContextKey).(SecurityUser)
	if !ok {
		return nil, false
	}
	return user, true
}

// GetAuthUserID 从上下文中提取认证用户ID
// ctx: 上下文
// 返回: 认证用户ID和是否存在的标志
func GetAuthUserID(ctx context.Context) uint32 {
	user, ok := ctx.Value(authUserContextKey).(SecurityUser)
	if !ok {
		return 0
	}
	ut64, err := strconv.ParseUint(user.GetSubject(), 10, 64)
	if err != nil {
		return 0
	}
	return uint32(ut64)
}

// GetAuthUserDomainID 从上下文中提取认证用户域ID
// ctx: 上下文
// 返回: 认证用户域ID和是否存在的标志
func GetAuthUserDomainID(ctx context.Context) uint32 {
	user, ok := ctx.Value(authUserContextKey).(SecurityUser)
	if !ok {
		return 0
	}
	domainID, err := strconv.ParseUint(user.GetDomain(), 10, 64)
	if err != nil {
		return 0
	}
	return uint32(domainID)
}

// GetSubject 获取主体标识
// 返回: 主体标识字符串
func (a *AuthClaims) GetSubject() string {
	if a == nil {
		return ""
	}
	if sub, ok := (*a)["sub"].(string); ok {
		return sub
	}
	return ""
}

// GetIssuer 获取签发者
// 返回: 签发者字符串
func (a *AuthClaims) GetIssuer() string {
	if a == nil {
		return ""
	}
	if iss, ok := (*a)["iss"].(string); ok {
		return iss
	}
	return ""
}

// GetID 获取唯一标识符
// 返回: 唯一标识符字符串
func (a *AuthClaims) GetID() string {
	if a == nil {
		return ""
	}
	if jti, ok := (*a)["jti"].(string); ok {
		return jti
	}
	return ""
}

// GetDomain 获取域
// 返回: 域字符串
func (a *AuthClaims) GetDomain() string {
	if a == nil {
		return ""
	}
	if domain, ok := (*a)["dom"].(string); ok {
		return domain
	}
	return ""
}

// GetExpiresAt 获取过期时间
// 返回: 过期时间
func (a *AuthClaims) GetExpiresAt() time.Time {
	if a == nil {
		return time.Time{}
	}
	switch exp := (*a)["exp"].(type) {
	case float64:
		return time.Unix(int64(exp), 0)
	case int64:
		return time.Unix(exp, 0)
	case time.Time:
		return exp
	default:
		return time.Time{}
	}
}

// IsExpired 检查是否已过期
// 返回: 是否已过期
func (a *AuthClaims) IsExpired() bool {
	exp := a.GetExpiresAt()
	if exp.IsZero() {
		return false
	}
	return time.Now().After(exp)
}
