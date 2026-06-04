package server

import (
	"backend-service/app/version/service/internal/conf"
	"backend-service/app/version/service/internal/service"
	pkgHealth "backend-service/pkg/health"
	"time"

	"backend-service/pkg/middleware/safelogging"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/ratelimit"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/http"
)

// NewHTTPServer new an HTTP server.
func NewHTTPServer(c *conf.Server, release *service.ReleaseService, checker pkgHealth.Checker, logger log.Logger) *http.Server {
	middlewares := []middleware.Middleware{safelogging.Server(logger)}
	if c.Http.Middleware != nil && c.Http.Middleware.Limiter != nil {
		middlewares = append(middlewares, ratelimit.Server())
	}
	middlewares = append(middlewares, recovery.Recovery())
	var opts = []http.ServerOption{
		http.Middleware(middlewares...),
	}
	if c.Http.Network != "" {
		opts = append(opts, http.Network(c.Http.Network))
	}
	if c.Http.Addr != "" {
		opts = append(opts, http.Address(c.Http.Addr))
	}
	if c.Http.Timeout != nil {
		opts = append(opts, http.Timeout(c.Http.Timeout.AsDuration()))
	}
	srv := http.NewServer(opts...)
	pkgHealth.RegisterHTTP(srv, checker, 2*time.Second)
	// v1.RegisterReleaseServiceHTTPServer(srv, release)
	return srv
}
