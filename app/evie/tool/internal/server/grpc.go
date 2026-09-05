// Package server · grpc.go
// evie/tool 的 gRPC transport 注册。
package server

import (
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/grpc"

	v1 "backend-service/api/evie/tool/v1"
	"backend-service/app/evie/tool/internal/conf"
	"backend-service/app/evie/tool/internal/data"
	"backend-service/app/evie/tool/internal/service"
)

// NewGRPCServer 创建 gRPC server。
func NewGRPCServer(
	c *conf.Server,
	cache *data.TokenCache,
	enhService *service.EnhancementService,
	asrService *service.ASRService,
	logger log.Logger,
) *grpc.Server {
	mws := []middleware.Middleware{recovery.Recovery()}
	if cache != nil {
		mws = append(mws, NewTokenAuthMiddleware(cache, nil))
	}
	_ = logger
	opts := []grpc.ServerOption{grpc.Middleware(mws...)}
	if c != nil && c.Grpc != nil {
		if c.Grpc.Network != "" {
			opts = append(opts, grpc.Network(c.Grpc.Network))
		}
		if c.Grpc.Addr != "" {
			opts = append(opts, grpc.Address(c.Grpc.Addr))
		}
		if c.Grpc.Timeout != nil {
			opts = append(opts, grpc.Timeout(c.Grpc.Timeout.AsDuration()))
		}
	}
	srv := grpc.NewServer(opts...)
	if enhService != nil {
		v1.RegisterEnhancementServiceServer(srv, enhService)
	}
	if asrService != nil {
		v1.RegisterASRServiceServer(srv, asrService)
	}
	return srv
}