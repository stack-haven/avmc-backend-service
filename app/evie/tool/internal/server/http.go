// Package server · http.go
// evie/tool 的 HTTP transport 注册。
//
// M0~M2 阶段：注册 Bearer Token 中间件。
// M6c 阶段：注册 EnhancementService（M7+ 追加 ASRService / AdminService）。
// M9 阶段：注册健康检查端点 /health/live + /health/ready。
package server

import (
	"context"
	"strings"
	"time"

	pkgHealth "backend-service/pkg/health"

	kvalidate "github.com/go-kratos/kratos/contrib/middleware/validate/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport"
	kratoshttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/gorilla/handlers"

	v1 "backend-service/api/evie/tool/v1"
	"backend-service/app/evie/tool/internal/conf"
	"backend-service/app/evie/tool/internal/data"
	"backend-service/app/evie/tool/internal/metrics"
	"backend-service/app/evie/tool/internal/service"
)

// skipPaths HTTP 跳过鉴权的路径（M9 收口时维护）。
// 默认空：所有 HTTP 接口都需通过 Bearer Token 鉴权。
var skipPaths = []string{}

// NewHTTPServer 创建 HTTP server。
//
//	c:              server config（addr / network / timeout）
//	cache:          Token 缓存（M2 注入）
//	enhService:     EnhancementService（M6c 注入）
//	asrService:     ASRService（M7 注入）
//	checker:        健康检查器（M9 注入）
//	logger:         kratos logger
func NewHTTPServer(
	c *conf.Server,
	cache data.TokenLookup,
	enhService *service.EnhancementService,
	asrService *service.ASRService,
	checker pkgHealth.Checker,
	logger log.Logger,
) *kratoshttp.Server {
	// v1.3 中间件顺序（外层 → 内层）：
	//   recovery → metrics → auth → validate
	//
	// 关键：auth 在 validate 之前，避免未鉴权请求探测验证规则
	// （无 token + 无效 body 应返 401 而非 400）。
	mws := []middleware.Middleware{recovery.Recovery(), metricsMiddleware()}
	if cache != nil {
		mws = append(mws, NewTokenAuthMiddleware(cache, skipPaths))
	}
	mws = append(mws, kvalidate.ProtoValidate())
	_ = logger

	// CORS 中间件（配置驱动，v1.3）。
	//
	// 为什么用 kratoshttp.Filter 而不是 kratoshttp.Middleware：
	//   浏览器对跨域非简单请求（如 application/json POST）会先发 OPTIONS 预检请求。
	//   如果 CORS 在 Middleware 链里，OPTIONS 请求会先被 TokenAuth 看到
	//   （无 Authorization 头）→ 返回 401 → 浏览器拒绝跨域响应。
	//   作为 Filter 时，CORS 在所有 Middleware / handler 之前执行，
	//   且 gorilla/handlers.CORS 自动识别 OPTIONS + Origin 头，
	//   直接返回 204 + CORS 响应头，不进入业务链。
	//
	// 配置（conf.Server.HTTP.cors_allowed_origins）：
	//   - 空（默认）：不启用 CORS（仅同源调用）
	//   - "*"：允许任意来源（仅 dev/test；生产禁止）
	//   - "https://app.example.com,https://admin.example.com"：白名单
	opts := []kratoshttp.ServerOption{}

	// 仅当配置了 cors_allowed_origins 才添加 CORS filter（默认不启用）
	if c != nil && c.Http != nil && c.Http.GetCorsAllowedOrigins() != "" {
		originsRaw := c.Http.GetCorsAllowedOrigins()
		var allowedOrigins []string
		if originsRaw == "*" {
			// dev/test: 允许任意来源；生产禁止
			allowedOrigins = []string{"*"}
		} else {
			// 逗号分隔的白名单
			for _, o := range strings.Split(originsRaw, ",") {
				o = strings.TrimSpace(o)
				if o != "" {
					allowedOrigins = append(allowedOrigins, o)
				}
			}
		}
		if len(allowedOrigins) > 0 {
			opts = append(opts, kratoshttp.Filter(handlers.CORS(
				handlers.AllowedHeaders([]string{
					"Content-Type",
					"Authorization",
					"X-Request-Id",
				}),
				handlers.AllowedMethods([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}),
				handlers.AllowedOrigins(allowedOrigins),
				handlers.ExposedHeaders([]string{
					"X-Request-Id",
				}),
			)))
		}
	}

	// v1.3 Rate Limiter（per-IP，默认关闭）。
	// 注：rate limit 作为 Filter 而非 Middleware，确保 OPTIONS 请求也限流（防 CORS preflight 绕过）。
	if c != nil && c.Http != nil && c.Http.GetRateLimitPerMinute() > 0 {
		limiter := newRateLimiter(int(c.Http.GetRateLimitPerMinute()), time.Minute, logger)
		opts = append(opts, kratoshttp.Filter(limiter.middleware))
	}

	opts = append(opts, kratoshttp.Middleware(mws...))
	if c != nil && c.Http != nil {
		if c.Http.Network != "" {
			opts = append(opts, kratoshttp.Network(c.Http.Network))
		}
		if c.Http.Addr != "" {
			opts = append(opts, kratoshttp.Address(c.Http.Addr))
		}
		if c.Http.Timeout != nil {
			opts = append(opts, kratoshttp.Timeout(c.Http.Timeout.AsDuration()))
		}
	}

	srv := kratoshttp.NewServer(opts...)
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
	// Metrics 文本导出（Prometheus 兼容，v1.3 加 IP 白名单）
	metricsHandler := metrics.Default.Handler()
	if c != nil && c.Http != nil && c.Http.GetMetricsAllowedIps() != "" {
		metricsHandler = newMetricsIPFilter(c.Http.GetMetricsAllowedIps()).middleware(metricsHandler)
	}
	srv.Handle("/metrics", metricsHandler)
	// pprof 调试端点（仅 EVIE_TOOL_PPROF=1）
	MountPProf(srv)
	return srv
}

// metricsMiddleware 记录 HTTP 请求计数与延迟到 metrics.Default。
//
// 错误码：返回 err 时 status=500；其它场景使用 transport 推断。
func metricsMiddleware() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			start := time.Now()
			reply, err = handler(ctx, req)
			path := "unknown"
			method := "unknown"
			if tr, ok := transport.FromServerContext(ctx); ok {
				path = tr.Operation()
				method = tr.Kind().String()
			}
			status := "200"
			if err != nil {
				status = "500"
			}
			metrics.HTTPRequestsTotal.Inc(method, path, status)
			metrics.RequestDuration.Observe(time.Since(start).Seconds(), method, path)
			return reply, err
		}
	}
}
