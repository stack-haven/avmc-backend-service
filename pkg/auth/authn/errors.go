package authn

import (
	"errors"

	"backend-service/pkg/auth/errs"
)

// ErrorCode 错误码类型（别名，统一到 errs.ErrorCode）。
type ErrorCode = errs.ErrorCode

// 错误码定义
const (
	// ErrCodeUnknown 未知错误
	ErrCodeUnknown ErrorCode = iota
	// ErrCodeInvalidToken 无效令牌
	ErrCodeInvalidToken
	// ErrCodeExpiredToken 令牌过期
	ErrCodeExpiredToken
	// ErrCodeInvalidSignature 无效签名
	ErrCodeInvalidSignature
	// ErrCodeInvalidClaims 无效声明
	ErrCodeInvalidClaims
	// ErrCodeMissingToken 缺少令牌
	ErrCodeMissingToken
	// ErrCodeUnsupportedTokenType 不支持的令牌类型
	ErrCodeUnsupportedTokenType
	// ErrCodeInvalidTokenFormat 无效的令牌格式
	ErrCodeInvalidTokenFormat
	// ErrCodeUnsupportedTokenScheme 不支持的令牌方案
	ErrCodeUnsupportedTokenScheme
	// ErrCodeNoTransportContext 无传输上下文
	ErrCodeNoTransportContext
	// ErrCodeInitializationFailed 初始化失败
	ErrCodeInitializationFailed
	// ErrCodeProviderNotFound 提供者未找到
	ErrCodeProviderNotFound
	// ErrCodeInvalidConfiguration 无效配置
	ErrCodeInvalidConfiguration
	// ErrCodeTokenCreationFailed 令牌创建失败
	ErrCodeTokenCreationFailed
	// ErrCodeTokenRefreshFailed 令牌刷新失败
	ErrCodeTokenRefreshFailed
	// ErrCodeTokenRevocationFailed 令牌撤销失败
	ErrCodeTokenRevocationFailed
	// ErrCodeInvalidSubject 无效主体
	ErrCodeInvalidSubject
	// ErrCodeInvalidIssuer 无效签发者
	ErrCodeInvalidIssuer
	// ErrCodeInvalidAudience 无效接收者
	ErrCodeInvalidAudience
	// ErrCodeNotBeforeTime 未到生效时间
	ErrCodeNotBeforeTime
)

// 预定义错误（纯字符串错误，供 errors.Is 判断使用）
var (
	ErrUnknown                = errors.New("unknown authentication error")
	ErrInvalidToken           = errors.New("invalid token")
	ErrExpiredToken           = errors.New("token has expired")
	ErrInvalidSignature       = errors.New("invalid token signature")
	ErrInvalidClaims          = errors.New("invalid token claims")
	ErrMissingToken           = errors.New("missing authentication token")
	ErrUnsupportedTokenType   = errors.New("unsupported token type")
	ErrInvalidTokenFormat     = errors.New("invalid token format")
	ErrUnsupportedTokenScheme = errors.New("unsupported token scheme")
	ErrNoTransportContext     = errors.New("no transport context found")
	ErrInitializationFailed   = errors.New("authenticator initialization failed")
	ErrProviderNotFound       = errors.New("authentication provider not found")
	ErrInvalidConfiguration   = errors.New("invalid authenticator configuration")
	ErrTokenCreationFailed    = errors.New("token creation failed")
	ErrTokenRefreshFailed     = errors.New("token refresh failed")
	ErrTokenRevocationFailed  = errors.New("token revocation failed")
	ErrInvalidSubject         = errors.New("invalid subject in token")
	ErrInvalidIssuer          = errors.New("invalid issuer in token")
	ErrInvalidAudience        = errors.New("invalid audience in token")
	ErrNotBeforeTime          = errors.New("token not valid yet")
)

// AuthError 认证错误类型（别名，统一到 errs.Error）。
type AuthError = errs.Error

// NewAuthError 创建认证错误（转发到统一错误结构）。
func NewAuthError(code ErrorCode, message string, err error) *AuthError {
	return errs.New(code, message, err)
}

// IsAuthError 检查错误是否为认证错误。
func IsAuthError(err error) bool {
	return errs.Is(err)
}

// GetAuthErrorCode 获取认证错误码。
func GetAuthErrorCode(err error) (ErrorCode, bool) {
	return errs.GetCode(err)
}
