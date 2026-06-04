package server

import (
	v1 "backend-service/api/version/service/v1"
	"backend-service/app/version/service/internal/conf"
	"backend-service/app/version/service/internal/service"

	"backend-service/pkg/middleware/safelogging"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/ratelimit"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/grpc"
)

// NewGRPCServer new a gRPC server.
func NewGRPCServer(c *conf.Server, release *service.ReleaseService, logger log.Logger) *grpc.Server {
	middlewares := []middleware.Middleware{safelogging.Server(logger)}
	if c.Grpc.Middleware != nil && c.Grpc.Middleware.Limiter != nil {
		middlewares = append(middlewares, ratelimit.Server())
	}
	middlewares = append(middlewares, recovery.Recovery())
	var opts = []grpc.ServerOption{
		grpc.Middleware(middlewares...),
	}
	if c.Grpc.Network != "" {
		opts = append(opts, grpc.Network(c.Grpc.Network))
	}
	if c.Grpc.Addr != "" {
		opts = append(opts, grpc.Address(c.Grpc.Addr))
	}
	if c.Grpc.Timeout != nil {
		opts = append(opts, grpc.Timeout(c.Grpc.Timeout.AsDuration()))
	}
	srv := grpc.NewServer(opts...)
	v1.RegisterReleaseServiceServer(srv, release)
	return srv
}
