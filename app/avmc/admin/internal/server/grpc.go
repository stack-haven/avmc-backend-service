package server

import (
	v1 "backend-service/api/avmc/admin/v1"
	"backend-service/app/avmc/admin/internal/conf"
	"backend-service/app/avmc/admin/internal/service"
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

func newGRPCWhiteListMatcher() selector.MatchFunc {
	whiteList := map[string]bool{
		v1.OperationAuthServiceLoginByPhoneCode: true,
		v1.OperationAuthServiceLoginPassword:    true,
		v1.OperationAuthServiceLoginByEmail:     true,
		v1.OperationAuthServiceLoginByUsername:  true,
		v1.OperationAuthServiceRefreshToken:     true,
	}
	return func(_ context.Context, operation string) bool {
		return !whiteList[operation]
	}
}

// NewGRPCServer new a gRPC server.
func NewGRPCServer(c *conf.Server,
	auth *service.AuthServiceService,
	user *service.UserServiceService,
	dept *service.DeptServiceService,
	menu *service.MenuServiceService,
	role *service.RoleServiceService,
	post *service.PostServiceService,
	project *service.ProjectServiceService,
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
			).Match(newGRPCWhiteListMatcher()).Build(),
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
	v1.RegisterAuthServiceServer(srv, auth)
	v1.RegisterUserServiceServer(srv, user)
	v1.RegisterDeptServiceServer(srv, dept)
	v1.RegisterMenuServiceServer(srv, menu)
	v1.RegisterRoleServiceServer(srv, role)
	v1.RegisterPostServiceServer(srv, post)
	v1.RegisterProjectServiceServer(srv, project)
	return srv
}
