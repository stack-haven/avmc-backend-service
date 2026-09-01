package server

import (
	"context"
	"time"

	v1 "backend-service/api/evie/service/v1"
	"backend-service/app/evie/service/internal/conf"
	"backend-service/app/evie/service/internal/service"
	"backend-service/pkg/audit"
	authzEngine "backend-service/pkg/auth/authz"
	"backend-service/pkg/auth/session"

	pkgHealth "backend-service/pkg/health"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	"github.com/go-kratos/kratos/v2/transport/http"
)

func newHTTPWhiteListMatcher() selector.MatchFunc {
	whiteList := map[string]bool{
		// TODO: 添加白名单操作
	}
	return func(_ context.Context, operation string) bool {
		return !whiteList[operation]
	}
}

func NewHTTPServer(
	c *conf.Server,
	logger log.Logger,
	authenticator *session.Manager,
	authorizer authzEngine.Enforcer,
	checker pkgHealth.Checker,
	dictionaryService *service.DictionaryServiceService,
	asrService *service.ASRServiceService,
	providerService *service.ProviderServiceService,
	enhancementService *service.EnhancementServiceService,
	auditClient audit.Client,
) (*http.Server, error) {
	if c == nil || c.Http == nil {
		return nil, nil
	}
	middlewares, err := newServerMiddleware(c.Http.Middleware, newHTTPWhiteListMatcher(), logger, authenticator, authorizer, auditClient)
	if err != nil {
		return nil, err
	}
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
	// WebSocket 流式识别入口（绕过 auth 中间件，service 层内部自行鉴权）
	srv.HandleFunc("/evie/v1/asr/stream", asrService.ServeHTTP)
	v1.RegisterDictionaryServiceHTTPServer(srv, dictionaryService)
	v1.RegisterASRServiceHTTPServer(srv, asrService)
	v1.RegisterProviderServiceHTTPServer(srv, providerService)
	v1.RegisterEnhancementServiceHTTPServer(srv, enhancementService)
	return srv, nil
}
