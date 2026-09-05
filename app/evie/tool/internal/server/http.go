// Package server · http.go
// evie/tool 的 HTTP transport 注册。
//
// M0~M2 阶段：注册 Bearer Token 中间件。
// M6c 阶段：注册 EnhancementService（M7+ 追加 ASRService / AdminService）。
// M9 阶段：注册健康检查端点 /health/live + /health/ready。
package server

import (
	"time"

	pkgHealth "backend-service/pkg/health"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/http"

	v1 "backend-service/api/evie/tool/v1"
	"backend-service/app/evie/tool/internal/conf"
	"backend-service/app/evie/tool/internal/data"
	"backend-service/app/evie/tool/internal/service"
)

// skipPaths HTTP 跳过鉴权的路径（M9 收口时维护）。
// 默认空：所有 HTTP 接口都需通过 Bearer Token 鉴权。
var skipPaths = []string{}

// NewHTTPServer 创建 HTTP server。
//
//   c:              server config（addr / network / timeout）
//   cache:          Token 缓存（M2 注入）
//   enhService:     EnhancementService（M6c 注入）
//   asrService:     ASRService（M7 注入）
//   checker:        健康检查器（M9 注入）
//   logger:         kratos logger
func NewHTTPServer(
	c *conf.Server,
	cache *data.TokenCache,
	enhService *service.EnhancementService,
	asrService *service.ASRService,
	checker pkgHealth.Checker,
	logger log.Logger,
) *http.Server {
	mws := []middleware.Middleware{recovery.Recovery()}
	if cache != nil {
		mws = append(mws, NewTokenAuthMiddleware(cache, skipPaths))
	}
	_ = logger

	opts := []http.ServerOption{http.Middleware(mws...)}
	if c != nil && c.Http != nil {
		if c.Http.Network != "" {
			opts = append(opts, http.Network(c.Http.Network))
		}
		if c.Http.Addr != "" {
			opts = append(opts, http.Address(c.Http.Addr))
		}
		if c.Http.Timeout != nil {
			opts = append(opts, http.Timeout(c.Http.Timeout.AsDuration()))
		}
	}

	srv := http.NewServer(opts...)
	// 注册 EnhancementService
	if enhService != nil {
		v1.RegisterEnhancementServiceHTTPServer(srv, enhService)
	}
	// 注册 ASRService（M7）
	if asrService != nil {
		v1.RegisterASRServiceHTTPServer(srv, asrService)
	}
	// 健康检查 endpoint（M9）
	if checker != nil {
		pkgHealth.RegisterHTTP(srv, checker, 2*time.Second)
	}
	return srv
}