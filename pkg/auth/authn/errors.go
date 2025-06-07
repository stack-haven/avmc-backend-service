package authn

import (
	"errors"
	"fmt"
)

// 错误码定义
type ErrorCode int

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

// 预定义错误
var (
	// ErrUnknown 未知错误
	ErrUnknown = errors.New("unknown authentication error")
	// ErrInvalidToken 无效令牌
	ErrInvalidToken = errors.New("invalid token")
	// ErrExpiredToken 令牌过期
	ErrExpiredToken = errors.New("token has expired")
	// ErrInvalidSignature 无效签名
	ErrInvalidSignature = errors.New("invalid token signature")
	// ErrInvalidClaims 无效声明
	ErrInvalidClaims = errors.New("invalid token claims")
	// ErrMissingToken 缺少令牌
	ErrMissingToken = errors.New("missing authentication token")
	// ErrUnsupportedTokenType 不支持的令牌类型
	ErrUnsupportedTokenType = errors.New("unsupported token type")
	// ErrInvalidTokenFormat 无效的令牌格式
	ErrInvalidTokenFormat = errors.New("invalid token format")
	// ErrUnsupportedTokenScheme 不支持的令牌方案
	ErrUnsupportedTokenScheme = errors.New("unsupported token scheme")
	// ErrNoTransportContext 无传输上下文
	ErrNoTransportContext = errors.New("no transport context found")
	// ErrInitializationFailed 初始化失败
	ErrInitializationFailed = errors.New("authenticator initialization failed")
	// ErrProviderNotFound 提供者未找到
	ErrProviderNotFound = errors.New("authentication provider not found")
	// ErrInvalidConfiguration 无效配置
	ErrInvalidConfiguration = errors.New("invalid authenticator configuration")
	// ErrTokenCreationFailed 令牌创建失败
	ErrTokenCreationFailed = errors.New("token creation failed")
	// ErrTokenRefreshFailed 令牌刷新失败
	ErrTokenRefreshFailed = errors.New("token refresh failed")
	// ErrTokenRevocationFailed 令牌撤销失败
	ErrTokenRevocationFailed = errors.New("token revocation failed")
	// ErrInvalidSubject 无效主体
	ErrInvalidSubject = errors.New("invalid subject in token")
	// ErrInvalidIssuer 无效签发者
	ErrInvalidIssuer = errors.New("invalid issuer in token")
	// ErrInvalidAudience 无效接收者
	ErrInvalidAudience = errors.New("invalid audience in token")
	// ErrNotBeforeTime 未到生效时间
	ErrNotBeforeTime = errors.New("token not valid yet")
)

// AuthError 认证错误类型
type AuthError struct {
	// Code 错误码
	Code ErrorCode
	// Message 错误消息
	Message string
	// Err 原始错误
	Err error
}

// Error 实现error接口
func (e *AuthError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("authentication error [code=%d]: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("authentication error [code=%d]: %s", e.Code, e.Message)
}

// Unwrap 解包错误
func (e *AuthError) Unwrap() error {
	return e.Err
}

// NewAuthError 创建新的认证错误
func NewAuthError(code ErrorCode, message string, err error) *AuthError {
	return &AuthError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// IsAuthError 检查错误是否为认证错误
func IsAuthError(err error) bool {
	var authErr *AuthError
	return errors.As(err, &authErr)
}

// GetAuthErrorCode 获取认证错误码
func GetAuthErrorCode(err error) (ErrorCode, bool) {
	var authErr *AuthError
	if errors.As(err, &authErr) {
		return authErr.Code, true
	}
	return ErrCodeUnknown, false
}
