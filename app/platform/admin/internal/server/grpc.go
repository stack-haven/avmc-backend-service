package server

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	"github.com/go-kratos/kratos/v2/transport/grpc"

	v1 "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/app/platform/admin/internal/conf"
	"backend-service/app/platform/admin/internal/service"
	"backend-service/pkg/auth"
	authzEngine "backend-service/pkg/auth/authz"
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
	tenant *service.TenantServiceService,
	user *service.UserServiceService,
	dept *service.DeptServiceService,
	menu *service.MenuServiceService,
	tenantMenuPermissionGroup *service.TenantMenuPermissionGroupServiceService,
	role *service.RoleServiceService,
	post *service.PostServiceService,
	project *service.ProjectServiceService,
	dictionary *service.DictionaryServiceService,
	operationLog *service.OperationLogServiceService,
	loginLog *service.LoginLogServiceService,
	session *service.SessionServiceService,
	parameter *service.ParameterServiceService,
	storageProvider *service.StorageProviderServiceService,
	storageConfig *service.StorageConfigService,
	fileCenter *service.FileCenterServiceService,
	notification *service.NotificationServiceService,
	asyncTask *service.AsyncTaskServiceService,
	authenticator *auth.AuthToken,
	authorizer authzEngine.Authorizer,
	operationAudit *biz.OperationLogUsecase,
	logger log.Logger,
) (*grpc.Server, error) {
	if c == nil || c.Grpc == nil {
		return nil, fmt.Errorf("grpc server config is required")
	}
	middlewares, err := newServerMiddleware(c.Grpc.Middleware, newGRPCWhiteListMatcher(), logger, authenticator, authorizer, operationAudit)
	if err != nil {
		return nil, fmt.Errorf("building grpc middleware: %w", err)
	}
	var opts = []grpc.ServerOption{grpc.Middleware(middlewares...)}
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
	v1.RegisterTenantServiceServer(srv, tenant)
	v1.RegisterUserServiceServer(srv, user)
	v1.RegisterDeptServiceServer(srv, dept)
	v1.RegisterMenuServiceServer(srv, menu)
	v1.RegisterTenantMenuPermissionGroupServiceServer(srv, tenantMenuPermissionGroup)
	v1.RegisterRoleServiceServer(srv, role)
	v1.RegisterPostServiceServer(srv, post)
	v1.RegisterProjectServiceServer(srv, project)
	v1.RegisterDictionaryServiceServer(srv, dictionary)
	v1.RegisterOperationLogServiceServer(srv, operationLog)
	v1.RegisterLoginLogServiceServer(srv, loginLog)
	v1.RegisterSessionServiceServer(srv, session)
	v1.RegisterParameterServiceServer(srv, parameter)
	v1.RegisterStorageProviderServiceServer(srv, storageProvider)
	v1.RegisterStorageConfigServiceServer(srv, storageConfig)
	v1.RegisterFileCenterServiceServer(srv, fileCenter)
	v1.RegisterNotificationServiceServer(srv, notification)
	v1.RegisterAsyncTaskServiceServer(srv, asyncTask)
	return srv, nil
}
