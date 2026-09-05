// Package server · middleware.go
// Bearer Token 中间件工厂 + 通用中间件辅助函数。
package server

import (
	"github.com/go-kratos/kratos/v2/middleware"

	"backend-service/app/evie/tool/internal/data"
)

// NewTokenAuthMiddleware 构造 Bearer 鉴权中间件（M2 阶段）。
//
//   cache:    data.TokenCache（提供 Bearer → AuthInfo 查询）
//   skipPath: 跳过路径前缀（如 /healthz）
//
// 注入 ctx：
//   - data.AuthInfoFromContext(ctx) → *AuthInfo（推荐，业务代码首选）
//   - authn.GetAuthUserID/GetAuthUserTenantID → string（authn.SecurityUser，截断风险）
func NewTokenAuthMiddleware(cache *data.TokenCache, skipPath []string) middleware.Middleware {
	return data.TokenAuthMiddleware(cache, skipPath)
}

// middlewareChain 把若干 middleware 串成 middleware.Middleware 切片。
// 仅用于把可能为 nil 的中间件统一过滤。
func middlewareChain(mws ...interface{}) []middleware.Middleware {
	out := make([]middleware.Middleware, 0, len(mws))
	for _, m := range mws {
		if mw, ok := m.(middleware.Middleware); ok {
			out = append(out, mw)
		}
	}
	return out
}