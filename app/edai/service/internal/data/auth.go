package data

import (
	"time"

	"backend-service/app/edai/service/internal/biz"
	"backend-service/app/edai/service/internal/runtimeconfig"
	"backend-service/pkg/auth"
	"backend-service/pkg/auth/authn"
	"backend-service/pkg/conf"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

// NewAuthenticator 用 pkg/auth 自签 JWT。
//
// 4-2 ADR-013 决策 4：EDAI 自持账号与签名密钥，不读平台配置、不做 Casbin 鉴权。
func NewAuthenticator(bc *conf.Bootstrap, logger log.Logger) (authn.Authenticator, error) {
	key, method, expires := runtimeconfig.AuthConfig(bc)
	if key == "" {
		return nil, runtimeconfig.ConfigError("server.http.middleware.auth.key is required (EDAI 自签 JWT)")
	}
	accessExpiration := time.Duration(expires) * time.Second
	return auth.NewAuthenticator(auth.AuthConfig{
		Key:              key,
		Method:           method,
		AccessExpiration: accessExpiration,
	}, authn.NewSecurity(logger))
}

// AuthProviderSet 认证相关依赖。
var AuthProviderSet = wire.NewSet(
	NewAuthenticator,
	wire.Bind(new(biz.Authenticator), new(authn.Authenticator)),
)
