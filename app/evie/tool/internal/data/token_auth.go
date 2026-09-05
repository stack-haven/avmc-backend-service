// Package data · token_auth.go
// Bearer Token 中间件：从 Authorization 头提取 token，查 Redis 反序列化为 AuthInfo，注入 ctx。
//
// 关键设计：
//  1. 鉴权完全基于 qua 的 Redis（oauth2_access_token:<token>），不引入 JWT 验签。
//  2. ID 字段全部以字符串存储（qua ID 常 > 2^32）；不依赖 pkg/auth/authn 的 uint32 accessor。
//  3. ctx 注入：biz.AuthContext（抽象）+ authn.SecurityUser（兼容）；通过 biz.WithAuth 注入。
//  4. 失败语义统一走 v1.ErrorTokenXxx（proto 生成）。
//  5. 跳过策略：路径前缀白名单（如 /healthz、/metrics）跳过中间件。
package data

import (
	"context"
	"errors"
	"fmt"
	"strings"

	v1 "backend-service/api/evie/tool/v1"

	"backend-service/app/evie/tool/internal/biz"
	"backend-service/pkg/auth/authn"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
)

// GetAccessToken / GetTenantID 让 *AuthInfo 实现 biz.AuthContext 接口。
func (a *AuthInfo) GetAccessToken() string { return a.AccessToken }
func (a *AuthInfo) GetTenantID() string    { return a.TenantID }

// TokenAuthMiddleware 构造一个 kratos 中间件。
//
//	cache:    提供 Bearer token → AuthInfo 的查询能力
//	skipPath: 跳过中间件的路径前缀（如 []string{"/healthz"}）
func TokenAuthMiddleware(cache *TokenCache, skipPath []string) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			// 0. 跳过白名单
			if tr, ok := transport.FromServerContext(ctx); ok {
				path := tr.Operation()
				for _, p := range skipPath {
					if p != "" && strings.HasPrefix(path, p) {
						return handler(ctx, req)
					}
				}
			}

			// 1. 提取 Bearer Token
			token, err := extractBearerToken(ctx)
			if err != nil {
				return nil, v1.ErrorTokenMissing("missing or malformed token: %v", err)
			}

			// 2. 查 Redis
			info, err := cache.Get(ctx, token)
			if err != nil {
				switch {
				case errors.Is(err, ErrTokenNotFound):
					return nil, v1.ErrorTokenInvalid("token not found in redis")
				case errors.Is(err, ErrTokenInvalid):
					return nil, v1.ErrorTokenInvalid("token value invalid json")
				default:
					return nil, v1.ErrorTokenLookupFailed("token lookup: %v", err)
				}
			}
			if info.TenantID == "" || info.UserID == "" {
				return nil, v1.ErrorTokenPayloadInvalid("token payload missing tenantId or userId")
			}

			// 3. 注入 ctx（biz 层抽象 + authn 兼容层）
			ctx = biz.WithAuth(ctx, info)
			ctx = authn.ContextWithAuthUser(ctx, &tokenSecurityUser{info: info})
			return handler(ctx, req)
		}
	}
}

// extractBearerToken 从 ctx 中取 Authorization: Bearer <token>。
// 复用 pkg/auth/authn.ParseContextToken：标准 "Bearer <token" 解析。
func extractBearerToken(ctx context.Context) (string, error) {
	return authn.ParseContextToken(authn.HeaderAuthorize, authn.BearerWord)(ctx)
}

// AuthInfoFromContext 从 ctx 取回 AuthInfo（业务代码首选）。
//
// 实现细节：通过 biz.AuthFrom 拿 ctx 值，再断言为 *AuthInfo。
// 保留此 API 是为了 service 层调用方不需重构。
func AuthInfoFromContext(ctx context.Context) (*AuthInfo, bool) {
	v, ok := biz.AuthFrom(ctx)
	if !ok {
		return nil, false
	}
	info, ok := v.(*AuthInfo)
	return info, ok
}

// WithAuthInfo 把 AuthInfo 注入 ctx（供测试 / service 层复用）。
//
// 实现细节：转发到 biz.WithAuth，data → biz 正向依赖。
func WithAuthInfo(ctx context.Context, info *AuthInfo) context.Context {
	return biz.WithAuth(ctx, info)
}

// MustAuthInfo 强制取 AuthInfo；找不到时 panic（用于已经过中间件的 handler 内部）。
func MustAuthInfo(ctx context.Context) *AuthInfo {
	v, ok := AuthInfoFromContext(ctx)
	if !ok {
		panic("evie/tool: AuthInfo missing from context (middleware not configured?)")
	}
	return v
}

var _ authn.SecurityUser = (*tokenSecurityUser)(nil) // 编译期断言
var _ biz.AuthContext = (*AuthInfo)(nil)             // 编译期断言：data.AuthInfo 满足 biz.AuthContext
// tokenSecurityUser 实现 authn.SecurityUser，复用 authn.ContextWith* 系列。
//
// 注意：GetSubject / GetTenant 返回字符串，避免被 strconv.ParseUint 截断为 uint32。
// ParseFromContext 复刻现有 securityUser 的语义：返回 nil 表示 info 合法。
type tokenSecurityUser struct{ info *AuthInfo }

func (u *tokenSecurityUser) Name() string        { return "evie-tool-token-user" }
func (u *tokenSecurityUser) GetSubject() string  { return u.info.UserID }
func (u *tokenSecurityUser) GetTenant() string   { return u.info.TenantID }
func (u *tokenSecurityUser) GetUserID() string   { return u.info.UserID }
func (u *tokenSecurityUser) GetTenantID() string { return u.info.TenantID }
func (u *tokenSecurityUser) GetObject() string   { return "" }
func (u *tokenSecurityUser) GetAction() string   { return "*" }

// ParseFromContext 兼容 authn.SecurityUser 接口；本工具不需要在中间件内重新计算 object/action。
func (u *tokenSecurityUser) ParseFromContext(ctx context.Context) error {
	if u.info == nil {
		return fmt.Errorf("evie/tool: nil AuthInfo in tokenSecurityUser")
	}
	if u.info.UserID == "" {
		return fmt.Errorf("evie/tool: empty userId in AuthInfo")
	}
	return nil
}
