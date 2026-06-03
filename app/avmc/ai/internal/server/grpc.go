package server

import (
	v1 "backend-service/api/avmc/ai/v1"
	"backend-service/app/avmc/ai/internal/conf"
	"backend-service/app/avmc/ai/internal/service"
	"backend-service/pkg/auth"
	authzEngine "backend-service/pkg/auth/authz"
	authMiddleware "backend-service/pkg/auth/middleware"
	"context"

	"github.com/go-kratos/kratos/contrib/middleware/validate/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	"github.com/go-kratos/kratos/v2/transport/grpc"
)

// NewGRPCServer new a gRPC server.
func NewGRPCServer(c *conf.Server,
	chat *service.ChatServiceService,
	authenticator *auth.AuthToken,
	authorizer authzEngine.Authorizer,
	logger log.Logger,
) *grpc.Server {
	var opts = []grpc.ServerOption{
		grpc.Middleware(
			logging.Server(logger),
			selector.Server(
				authMiddleware.AuthnMiddleware(authenticator),
				authMiddleware.AuthzMiddleware(authorizer),
			).Match(func(_ context.Context, _ string) bool { return true }).Build(),
			validate.ProtoValidate(),
			recovery.Recovery(),
		),
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
	v1.RegisterChatServiceServer(srv, chat)
	return srv
}
