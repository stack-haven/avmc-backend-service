package server

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	"github.com/go-kratos/kratos/v2/transport/grpc"

	coreV1 "backend-service/api/core/service/v1"
	v1 "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/app/platform/admin/internal/conf"
	"backend-service/app/platform/admin/internal/service"
	"backend-service/pkg/auth"
	authzEngine "backend-service/pkg/auth/authz"
	authMiddleware "backend-service/pkg/auth/middleware"
)

func newGRPCWhiteListMatcher() selector.MatchFunc {
	whiteList := map[string]bool{
		v1.OperationAuthServiceLoginByPhoneCode: true,
		v1.OperationAuthServiceLoginPassword:    true,
		v1.OperationAuthServiceLoginByEmail:     true,
		v1.OperationAuthServiceLoginByUsername:  true,
		v1.OperationAuthServiceRefreshToken:     true,
		v1.OperationAuthServiceLogout:           true,
		// 跨服务鉴权委托：产品服务（evie 等）调用平台 IsAuthorized，
		// 该内部 RPC 不携带原始 JWT，需跳过本服务的认证/鉴权中间件。
		coreV1.AuthService_IsAuthorized_FullMethodName: true,
		// 跨服务审计委托：产品服务写入操作审计，同样不携带原始 JWT。
		coreV1.OperationLogService_CreateOperationLog_FullMethodName: true,
		// 跨服务文件中心：产品服务转发 JWT，走 crossServiceMatcher 仅认证（免鉴权）。
		coreV1.FileCenterService_CreateFileUploadSession_FullMethodName: true,
		coreV1.FileCenterService_UploadFileContent_FullMethodName:       true,
		coreV1.FileCenterService_ConfirmFileUpload_FullMethodName:       true,
		coreV1.FileCenterService_PresignFileDownload_FullMethodName:     true,
		coreV1.FileCenterService_DownloadFileContent_FullMethodName:     true,
	}
	return func(_ context.Context, operation string) bool {
		return !whiteList[operation]
	}
}

// newGRPCAuthzMatcher 返回 gRPC 鉴权白名单：认证白名单 + 自服务接口跳过 Casbin 鉴权。
func newGRPCAuthzMatcher() selector.MatchFunc {
	authn := newGRPCWhiteListMatcher()
	return func(ctx context.Context, operation string) bool {
		if !authn(ctx, operation) {
			return false
		}
		if isSelfServiceOperation(operation) {
			return false
		}
		return true
	}
}

// crossServiceMatcher 匹配转发 JWT 的跨服务 RPC，仅做认证（不做 Casbin 鉴权）。
func crossServiceMatcher() selector.MatchFunc {
	crossService := map[string]bool{
		coreV1.FileCenterService_CreateFileUploadSession_FullMethodName: true,
		coreV1.FileCenterService_UploadFileContent_FullMethodName:       true,
		coreV1.FileCenterService_ConfirmFileUpload_FullMethodName:       true,
		coreV1.FileCenterService_PresignFileDownload_FullMethodName:     true,
		coreV1.FileCenterService_DownloadFileContent_FullMethodName:     true,
	}
	return func(_ context.Context, operation string) bool {
		return crossService[operation]
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
	notificationProvider *service.NotificationProviderServiceService,
	device *service.DeviceServiceService,
	asyncTask *service.AsyncTaskServiceService,
	authz *service.AuthzService,
	coreOperationLog *service.CoreOperationLogService,
	coreFileCenter *service.CoreFileCenterService,
	authenticator *auth.AuthToken,
	authorizer authzEngine.Authorizer,
	operationAudit *biz.OperationLogUsecase,
	logger log.Logger,
) (*grpc.Server, error) {
	if c == nil || c.Grpc == nil {
		return nil, fmt.Errorf("grpc server config is required")
	}
	middlewares, err := newServerMiddleware(c.Grpc.Middleware, newGRPCWhiteListMatcher(), newGRPCAuthzMatcher(), logger, authenticator, authorizer, operationAudit)
	if err != nil {
		return nil, fmt.Errorf("building grpc middleware: %w", err)
	}
	// 跨服务认证：转发 JWT 的文件中心 RPC 仅做认证，认证通过后 tenant 从 JWT 提取
	middlewares = append(middlewares,
		selector.Server(
			authMiddleware.AuthnMiddleware(authenticator),
		).Match(crossServiceMatcher()).Build(),
	)
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
	coreV1.RegisterAuthServiceServer(srv, authz)
	coreV1.RegisterOperationLogServiceServer(srv, coreOperationLog)
	coreV1.RegisterFileCenterServiceServer(srv, coreFileCenter)
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
	v1.RegisterNotificationProviderServiceServer(srv, notificationProvider)
	v1.RegisterDeviceServiceServer(srv, device)
	v1.RegisterAsyncTaskServiceServer(srv, asyncTask)
	return srv, nil
}
