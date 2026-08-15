package server

import (
	"context"
	"fmt"
	nethttp "net/http"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/gorilla/handlers"

	v1 "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/cmd/server/assets"
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/app/platform/admin/internal/conf"
	"backend-service/app/platform/admin/internal/service"
	"backend-service/pkg/auth"
	authzEngine "backend-service/pkg/auth/authz"
	pkgHealth "backend-service/pkg/health"
)

// NewWhiteListMatcher 创建jwt白名单
func newHTTPWhiteListMatcher() selector.MatchFunc {
	whiteList := make(map[string]bool)
	whiteList[v1.OperationAuthServiceLoginByPhoneCode] = true
	whiteList[v1.OperationAuthServiceLoginPassword] = true
	whiteList[v1.OperationAuthServiceLoginByEmail] = true
	whiteList[v1.OperationAuthServiceLoginByUsername] = true
	whiteList[v1.OperationAuthServiceRefreshToken] = true
	whiteList[v1.OperationAuthServiceLogout] = true
	whiteList[v1.OperationTenantServiceListTenantSimples] = true
	return func(ctx context.Context, operation string) bool {
		if _, ok := whiteList[operation]; ok {
			return false
		}
		return true
	}
}

// newHTTPAuthzMatcher 返回鉴权白名单：白名单（登录/刷新等）+ 自服务接口（profile/codes 等）
// 跳过 Casbin 鉴权；其余操作都需要菜单权限。
func newHTTPAuthzMatcher() selector.MatchFunc {
	authn := newHTTPWhiteListMatcher()
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

// NewHTTPServer new an HTTP server.
func NewHTTPServer(c *conf.Server, logger log.Logger,
	authenticator *auth.AuthToken, authorizer authzEngine.Authorizer,
	checker pkgHealth.Checker,
	operationAudit *biz.OperationLogUsecase,
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
) (*http.Server, error) {
	if c == nil || c.Http == nil {
		return nil, fmt.Errorf("http server config is required")
	}
	if c.Http.Cors == nil {
		return nil, fmt.Errorf("http cors config is required")
	}
	middlewares, err := newServerMiddleware(c.Http.Middleware, newHTTPWhiteListMatcher(), newHTTPAuthzMatcher(), logger, authenticator, authorizer, operationAudit)
	if err != nil {
		return nil, fmt.Errorf("building http middleware: %w", err)
	}
	var opts = []http.ServerOption{
		http.Filter(handlers.CORS(
			handlers.AllowedHeaders(c.Http.Cors.Headers),
			handlers.AllowedMethods(c.Http.Cors.Methods),
			handlers.AllowedOrigins(c.Http.Cors.Origins),
		)),
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
	v1.RegisterAuthServiceHTTPServer(srv, auth)
	v1.RegisterTenantServiceHTTPServer(srv, tenant)
	v1.RegisterUserServiceHTTPServer(srv, user)
	v1.RegisterDeptServiceHTTPServer(srv, dept)
	v1.RegisterMenuServiceHTTPServer(srv, menu)
	v1.RegisterTenantMenuPermissionGroupServiceHTTPServer(srv, tenantMenuPermissionGroup)
	v1.RegisterRoleServiceHTTPServer(srv, role)
	v1.RegisterPostServiceHTTPServer(srv, post)
	v1.RegisterProjectServiceHTTPServer(srv, project)
	v1.RegisterDictionaryServiceHTTPServer(srv, dictionary)
	v1.RegisterOperationLogServiceHTTPServer(srv, operationLog)
	v1.RegisterLoginLogServiceHTTPServer(srv, loginLog)
	v1.RegisterSessionServiceHTTPServer(srv, session)
	v1.RegisterParameterServiceHTTPServer(srv, parameter)
	v1.RegisterStorageProviderServiceHTTPServer(srv, storageProvider)
	v1.RegisterStorageConfigServiceHTTPServer(srv, storageConfig)
	v1.RegisterFileCenterServiceHTTPServer(srv, fileCenter)
	v1.RegisterNotificationServiceHTTPServer(srv, notification)
	v1.RegisterNotificationProviderServiceHTTPServer(srv, notificationProvider)
	v1.RegisterDeviceServiceHTTPServer(srv, device)
	v1.RegisterAsyncTaskServiceHTTPServer(srv, asyncTask)
	if c.GetHttp().GetEnableSwagger() {
		allFS := nethttp.FS(assets.OpenAPIData)
		// swagger-ui: http://127.0.0.1:8000/docs/swagger-ui
		// swagger-ui: http://127.0.0.1:8000/docs/openapi.yaml
		srv.HandlePrefix("/docs", nethttp.StripPrefix("/docs/", nethttp.FileServer(allFS)))
	}
	return srv, nil
}
