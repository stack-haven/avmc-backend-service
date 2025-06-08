package authn

import (
	"context"
)

var _ Authenticator = (*NoopAuthenticator)(nil)

// NoopAuthenticator Noop认证器实现
type NoopAuthenticator struct {
}

// NoopAuthn Noop认证提供者
type NoopAuthn struct{}

// Name 获取提供者名称
func (p *NoopAuthn) Name() string {
	return "noop"
}

// NewAuthenticator 创建新的认证器实例
func (p *NoopAuthn) NewAuthenticator(ctx context.Context, opts ...Option) (Authenticator, error) {
	return new(NoopAuthenticator), nil
}

// Authenticate 验证用户身份并返回认证声明
func (a *NoopAuthenticator) Authenticate(ctx context.Context) (*AuthClaims, error) {
	return nil, nil
}

// Init 初始化认证器
func (a *NoopAuthenticator) Init(ctx context.Context, opts ...Option) error {
	return nil
}

// ValidateToken 验证令牌的有效性
func (a *NoopAuthenticator) ValidateToken(ctx context.Context, tokenString string) (*AuthClaims, error) {
	return nil, nil
}

// CreateToken 创建新的身份令牌
func (a *NoopAuthenticator) CreateToken(ctx context.Context, claims AuthClaims) (string, error) {
	return "", nil
}

// RefreshToken 刷新令牌，延长有效期
func (a *NoopAuthenticator) RefreshToken(ctx context.Context, token string) (string, error) {
	return "", nil
}

// RevokeToken 撤销令牌，使其失效
func (a *NoopAuthenticator) RevokeToken(ctx context.Context, token string) error {

	return nil
}

// Close 关闭认证器，释放资源
func (a *NoopAuthenticator) Close() error {
	// 无需特殊清理
	return nil
}

// Name 返回认证提供者的名称
func (a *NoopAuthenticator) Name() string {
	return "jwt"
}

// NewProvider 创建新的JWT认证提供者
func NewProvider() AuthProvider {
	return &NoopAuthn{}
}
