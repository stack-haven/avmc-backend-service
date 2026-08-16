package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-kratos/kratos/contrib/middleware/validate/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/ratelimit"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	"github.com/google/wire"

	v1 "backend-service/api/platform/service/v1"
	"backend-service/app/platform/service/internal/biz"
	"backend-service/app/platform/service/internal/conf"
	authzEngine "backend-service/pkg/auth/authz"
	authMiddleware "backend-service/pkg/auth/middleware"
	"backend-service/pkg/auth/session"
	"backend-service/pkg/middleware/safelogging"
)

// ProviderSet is server providers.
var ProviderSet = wire.NewSet(NewHTTPServer, NewGRPCServer, NewAsyncTaskWorker)

// anthnWhiteListOperations 认证白名单：跳过 JWT 认证的接口。
// 仅含登录前接口（无 token）；跨服务委托接口（IsAuthorized 等）现已携带原始 JWT，
// 不再跳过认证（保留在鉴权白名单中，见 anthzWhiteListOperations）。
var anthnWhiteListOperations = map[string]bool{
	v1.OperationAuthServiceLoginByPhoneCode:    true,
	v1.OperationAuthServiceLoginPassword:       true,
	v1.OperationAuthServiceLoginByEmail:        true,
	v1.OperationAuthServiceLoginByUsername:     true,
	v1.OperationAuthServiceRefreshToken:        true,
	v1.OperationAuthServiceLogout:              true,
	v1.OperationTenantServiceListTenantSimples: true,
}

// anthzWhiteListOperations 是“无需 Casbin 鉴权”的接口：
//  1. 登录前接口（无需 token，认证白名单也包含，此处一并跳过鉴权）
//  2. 自服务接口（当前登录用户查询自己的信息）
//  3. 跨服务委托接口（产品服务转发 JWT 或不携带原始 JWT）
var anthzWhiteListOperations = map[string]bool{
	// 登录前接口（与认证白名单一致，跳过鉴权）
	v1.OperationAuthServiceLoginByPhoneCode:    true,
	v1.OperationAuthServiceLoginPassword:       true,
	v1.OperationAuthServiceLoginByEmail:        true,
	v1.OperationAuthServiceLoginByUsername:     true,
	v1.OperationAuthServiceRefreshToken:        true,
	v1.OperationAuthServiceLogout:              true,
	v1.OperationTenantServiceListTenantSimples: true,
	// 自服务接口（认证通过即可访问）
	v1.OperationAuthServiceCodes:                              true,
	v1.OperationAuthServiceMenus:                              true,
	v1.OperationAuthServiceProfile:                            true,
	v1.OperationAuthServiceVbenProfile:                        true,
	v1.OperationSessionServiceListMySessions:                  true,
	v1.OperationNotificationServiceListMyNotifications:        true,
	v1.OperationNotificationServiceCountMyUnreadNotifications: true,
	v1.OperationNotificationServiceMarkNotificationRead:       true,
	v1.OperationNotificationServiceMarkNotificationsRead:      true,
	v1.OperationMenuServiceExistMenuByName:                    true,
	v1.OperationMenuServiceExistMenuByPath:                    true,
	v1.OperationRoleServiceExistRoleByName:                    true,
	// 跨服务鉴权委托：产品服务（evie 等）调用平台 IsAuthorized，
	// 该内部 RPC 不携带原始 JWT，需跳过本服务的认证/鉴权中间件。
	v1.AuthService_IsAuthorized_FullMethodName: true,
	// 跨服务审计委托：产品服务写入操作审计，同样不携带原始 JWT。
	v1.OperationLogService_CreateOperationLog_FullMethodName: true,
	// 跨服务文件中心：产品服务转发 JWT，走 crossServiceMatcher 仅认证（免鉴权）。
	v1.FileCenterService_CreateFileUploadSession_FullMethodName: true,
	v1.FileCenterService_UploadFileContent_FullMethodName:       true,
	v1.FileCenterService_ConfirmFileUpload_FullMethodName:       true,
	v1.FileCenterService_PresignFileDownload_FullMethodName:     true,
	v1.FileCenterService_DownloadFileContent_FullMethodName:     true,
}

// newAuthnWhiteListMatcher 认证白名单：白名单内（登录/刷新/登出等）跳过认证返回 false，
// 其余接口执行认证返回 true。
func newAuthnWhiteListMatcher() selector.MatchFunc {
	return func(ctx context.Context, operation string) bool {
		if _, ok := anthnWhiteListOperations[operation]; ok {
			return false
		}
		return true
	}
}

// newAuthzWhiteListMatcher 鉴权白名单：白名单内（自服务接口等）跳过 Casbin 鉴权返回 false，
// 其余接口执行鉴权返回 true。
func newAuthzWhiteListMatcher() selector.MatchFunc {
	authnMatcher := newAuthnWhiteListMatcher()
	return func(ctx context.Context, operation string) bool {
		// 认证跳过的接口，鉴权必然跳过（无 claims 无法鉴权）
		if !authnMatcher(ctx, operation) {
			return false
		}
		// 自服务接口等额外跳过鉴权
		if _, ok := anthzWhiteListOperations[operation]; ok {
			return false
		}
		return true
	}
}

func newServerMiddleware(
	cfg *conf.Middleware,
	authnMatcher selector.MatchFunc,
	authzMatcher selector.MatchFunc,
	logger log.Logger,
	authenticator *session.Manager,
	authorizer authzEngine.Authorizer,
	operationLog *biz.OperationLogUsecase,
) ([]middleware.Middleware, error) {
	if cfg == nil {
		return nil, fmt.Errorf("server middleware config is required")
	}

	var ms []middleware.Middleware
	if cfg.EnableRecovery {
		// Recovery wraps the remaining chain so panics from middleware and handlers are contained.
		ms = append(ms, recovery.Recovery())
	}
	if cfg.EnableLogging {
		ms = append(ms, safelogging.Server(logger))
	}

	limiterName := ""
	if cfg.Limiter != nil {
		limiterName = strings.ToLower(strings.TrimSpace(cfg.Limiter.Name))
	}
	switch limiterName {
	case "", "off", "none", "disabled":
	case "bbr":
		ms = append(ms, ratelimit.Server())
	default:
		return nil, fmt.Errorf("unsupported rate limiter %q", limiterName)
	}
	ms = append(ms,
		selector.Server(authMiddleware.AuthnMiddleware(authenticator)).Match(authnMatcher).Build(),
		selector.Server(authMiddleware.AuthzMiddleware(authorizer)).Match(authzMatcher).Build(),
		platformControlServer(),
	)
	if operationLog != nil {
		ms = append(ms, operationAuditServer(operationLog, logger))
	}
	if cfg.EnableValidate {
		ms = append(ms, validate.ProtoValidate())
	}
	return ms, nil
}
