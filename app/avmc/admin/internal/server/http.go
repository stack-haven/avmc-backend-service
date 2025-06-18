package server

import (
	v1 "backend-service/api/avmc/admin/v1"
	"backend-service/app/avmc/admin/cmd/server/assets"
	"backend-service/app/avmc/admin/internal/conf"
	"backend-service/app/avmc/admin/internal/service"
	"context"

	nethttp "net/http"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/gorilla/handlers"

	authnEngine "backend-service/pkg/auth/authn"

	authzEngine "backend-service/pkg/auth/authz"
	authMiddleware "backend-service/pkg/auth/middleware"
)

// NewWhiteListMatcher 创建jwt白名单
func newHTTPWhiteListMatcher() selector.MatchFunc {
	whiteList := make(map[string]bool)
	whiteList[v1.OperationAuthServiceLogin] = true
	whiteList[v1.OperationAuthServiceRefreshToken] = true
	return func(ctx context.Context, operation string) bool {
		if _, ok := whiteList[operation]; ok {
			return false
		}
		return true
	}
}

// NewMiddleware 创建中间件
func newHTTPMiddleware(
	logger log.Logger,
	authenticator authnEngine.Authenticator,
	authorizer authzEngine.Authorizer,
) []middleware.Middleware {
	var ms []middleware.Middleware
	ms = append(ms, logging.Server(logger))
	ms = append(ms, selector.Server(
		authMiddleware.AuthnMiddleware(authenticator),
		// auth.Server(userToken),
		authMiddleware.AuthzMiddleware(authorizer),
	).Match(newHTTPWhiteListMatcher()).Build())

	return ms
}

// NewHTTPServer new an HTTP server.
func NewHTTPServer(c *conf.Server, logger log.Logger,
	authenticator authnEngine.Authenticator, authorizer authzEngine.Authorizer,
	auth *service.AuthServiceService,
	user *service.UserServiceService,
	dept *service.DeptServiceService,
	menu *service.MenuServiceService,
	role *service.RoleServiceService,
	post *service.PostServiceService,
) *http.Server {
	var opts = []http.ServerOption{
		http.Filter(handlers.CORS(
			handlers.AllowedHeaders(c.Http.Cors.Headers),
			handlers.AllowedMethods(c.Http.Cors.Methods),
			handlers.AllowedOrigins(c.Http.Cors.Origins),
		)),
		http.Middleware(newHTTPMiddleware(logger, authenticator, authorizer)...),
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
	v1.RegisterAuthServiceHTTPServer(srv, auth)
	v1.RegisterUserServiceHTTPServer(srv, user)
	v1.RegisterDeptServiceHTTPServer(srv, dept)
	v1.RegisterMenuServiceHTTPServer(srv, menu)
	v1.RegisterRoleServiceHTTPServer(srv, role)
	v1.RegisterPostServiceHTTPServer(srv, post)
	if c.GetHttp().GetEnableSwagger() {
		allFS := nethttp.FS(assets.OpenApiData)
		// swagger-ui: http://127.0.0.1:8000/docs/swagger-ui
		// swagger-ui: http://127.0.0.1:8000/docs/openapu.yaml
		srv.HandlePrefix("/docs", nethttp.StripPrefix("/docs/", nethttp.FileServer(allFS)))
	}
	return srv
}
