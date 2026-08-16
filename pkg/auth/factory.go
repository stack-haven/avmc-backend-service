package auth

import (
	"context"
	"fmt"
	"time"

	"backend-service/pkg/auth/authn"
	authnJwt "backend-service/pkg/auth/authn/jwt"
)

// AuthConfig 认证配置，由各服务从自己的 conf 提取后传入。
// 所有服务必须共享同一把 Key（统一环境变量注入），否则跨服务 token 验证失败。
type AuthConfig struct {
	Key               string        // JWT 签名密钥（所有服务共享）
	Method            string        // 签名方法，默认 HS256
	AccessExpiration  time.Duration // 访问令牌过期时间，默认 7 天
	RefreshExpiration time.Duration // 刷新令牌过期时间，默认 = AccessExpiration * 10
}

// NewAuthenticator 根据统一配置创建 JWT 认证器（本地验签，无状态验证）。
//
// 认证工厂统一收敛在此，各服务只需提取自身 conf 的 auth 配置并调用，
// 避免复制 30 行 JWT 组装代码。会话管理（AuthToken）与 SecurityUser 工厂
// （AuthSecurity）本身已公共，见 auth_token.go / auth_security.go。
func NewAuthenticator(cfg AuthConfig, security *AuthSecurity) (authn.Authenticator, error) {
	if security == nil {
		return nil, fmt.Errorf("auth security is required")
	}
	if cfg.Key == "" {
		return nil, fmt.Errorf("auth signing key is required")
	}
	if cfg.Method == "" {
		cfg.Method = "HS256"
	}
	if cfg.AccessExpiration <= 0 {
		cfg.AccessExpiration = 7 * 24 * time.Hour
	}
	if cfg.RefreshExpiration <= 0 {
		cfg.RefreshExpiration = cfg.AccessExpiration * 10
	}

	provider := authnJwt.NewProvider()
	authenticator, err := provider.NewAuthenticator(
		context.Background(),
		authn.WithSigningKey([]byte(cfg.Key)),
		authn.WithSigningMethod(cfg.Method),
		authn.WithTokenExpiration(cfg.AccessExpiration),
		authn.WithRefreshTokenExpiration(cfg.RefreshExpiration),
		authn.WithUserFactory(security.NewSecurityUser),
	)
	if err != nil {
		return nil, fmt.Errorf("creating authenticator: %w", err)
	}
	return authenticator, nil
}
