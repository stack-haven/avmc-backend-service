package server

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	"github.com/google/wire"

	"backend-service/app/evie/service/internal/conf"
	"backend-service/pkg/audit"
	auditMiddleware "backend-service/pkg/audit/middleware"
	"backend-service/pkg/auth"
	authzEngine "backend-service/pkg/auth/authz"
	authMiddleware "backend-service/pkg/auth/middleware"
	"backend-service/pkg/middleware/safelogging"
)

// ProviderSet is server providers.
var ProviderSet = wire.NewSet(NewHTTPServer, NewGRPCServer)

func newServerMiddleware(
	cfg *conf.Middleware,
	matcher selector.MatchFunc,
	logger log.Logger,
	authenticator *auth.AuthToken,
	authorizer authzEngine.Enforcer,
	auditClient audit.Client,
) ([]middleware.Middleware, error) {
	if cfg == nil {
		return nil, fmt.Errorf("server middleware config is required")
	}

	var ms []middleware.Middleware
	if cfg.EnableRecovery {
		ms = append(ms, recovery.Recovery())
	}
	if cfg.EnableLogging {
		ms = append(ms, safelogging.Server(logger))
	}

	ms = append(ms,
		selector.Server(
			authMiddleware.AuthnMiddleware(authenticator),
			authMiddleware.AuthzMiddleware(authorizer),
		).Match(matcher).Build(),
	)
	ms = append(ms, auditMiddleware.Server(auditClient, audit.AuthnExtractor, logger))
	return ms, nil
}
