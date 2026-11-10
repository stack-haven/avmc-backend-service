package server

import (
	"context"
	"time"

	v1 "backend-service/api/edai/service/v1"
	"backend-service/app/edai/service/internal/conf"
	"backend-service/app/edai/service/internal/service"
	"backend-service/pkg/auth/authn"
	authMiddleware "backend-service/pkg/auth/middleware"
	pkgHealth "backend-service/pkg/health"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/google/wire"
)

// ProviderSet 服务端依赖注入集合。
var ProviderSet = wire.NewSet(NewHTTPServer, NewGRPCServer)

// 免鉴权白名单：登录、刷新、退出、健康检查、系统信息（本机只读）。
// 注意：白名单按 kratos operation 名匹配，不是 URL 路径。
//
// Logout 必须在白名单内：访问令牌过期时用户仍需能撤销刷新令牌，
// 否则「退出登录」在最需要它的时候恰好不可用。
var authWhitelist = map[string]bool{
	"/edai.service.v1.AuthService/Login":           true,
	"/edai.service.v1.AuthService/RefreshToken":    true,
	"/edai.service.v1.AuthService/Logout":          true,
	"/edai.service.v1.SystemService/GetSystemInfo": true,
}

func authMatcher() selector.MatchFunc {
	return func(_ context.Context, operation string) bool {
		return !authWhitelist[operation]
	}
}

// buildMiddlewares 组装中间件链。
//
// 顺序：recovery → 日志（由 kratos 默认 logging 提供）→ 鉴权。
// EDAI 不做 Casbin 鉴权（4-2 ADR-013）：角色断言在 biz 层按需执行。
func buildMiddlewares(cfg *conf.Middleware, logger log.Logger, authenticator authn.Authenticator) ([]middleware.Middleware, error) {
	ms := []middleware.Middleware{recovery.Recovery()}
	ms = append(ms,
		selector.Server(authMiddleware.AuthnMiddleware(authenticator)).
			Match(authMatcher()).
			Build(),
	)
	return ms, nil
}

// NewHTTPServer 创建 HTTP 服务。
func NewHTTPServer(
	c *conf.Server,
	logger log.Logger,
	authenticator authn.Authenticator,
	checker pkgHealth.Checker,
	authService *service.AuthServiceService,
	sourceService *service.SourceServiceService,
	fileService *service.FileServiceService,
	scanService *service.ScanServiceService,
	systemService *service.SystemServiceService,
) (*http.Server, error) {
	if c == nil || c.Http == nil {
		return nil, nil
	}
	ms, err := buildMiddlewares(c.Http.Middleware, logger, authenticator)
	if err != nil {
		return nil, err
	}
	opts := []http.ServerOption{http.Middleware(ms...)}
	if c.Http.Network != "" {
		opts = append(opts, http.Network(c.Http.Network))
	}
	if c.Http.Addr != "" {
		opts = append(opts, http.Address(c.Http.Addr))
	}
	if c.Http.Timeout != nil && c.Http.Timeout.AsDuration() > 0 {
		opts = append(opts, http.Timeout(c.Http.Timeout.AsDuration()))
	}
	srv := http.NewServer(opts...)

	pkgHealth.RegisterHTTP(srv, checker, 2*time.Second)
	v1.RegisterAuthServiceHTTPServer(srv, authService)
	v1.RegisterSourceServiceHTTPServer(srv, sourceService)
	v1.RegisterFileServiceHTTPServer(srv, fileService)
	v1.RegisterScanServiceHTTPServer(srv, scanService)
	v1.RegisterSystemServiceHTTPServer(srv, systemService)
	return srv, nil
}

// NewGRPCServer 创建 gRPC 服务。
func NewGRPCServer(
	c *conf.Server,
	logger log.Logger,
	authenticator authn.Authenticator,
	authService *service.AuthServiceService,
	sourceService *service.SourceServiceService,
	fileService *service.FileServiceService,
	scanService *service.ScanServiceService,
	systemService *service.SystemServiceService,
) (*grpc.Server, error) {
	if c == nil || c.Grpc == nil {
		return nil, nil
	}
	ms, err := buildMiddlewares(c.Grpc.Middleware, logger, authenticator)
	if err != nil {
		return nil, err
	}
	opts := []grpc.ServerOption{grpc.Middleware(ms...)}
	if c.Grpc.Addr != "" {
		opts = append(opts, grpc.Address(c.Grpc.Addr))
	}
	srv := grpc.NewServer(opts...)
	v1.RegisterAuthServiceServer(srv, authService)
	v1.RegisterSourceServiceServer(srv, sourceService)
	v1.RegisterFileServiceServer(srv, fileService)
	v1.RegisterScanServiceServer(srv, scanService)
	v1.RegisterSystemServiceServer(srv, systemService)
	return srv, nil
}
