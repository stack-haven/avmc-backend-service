package server

import (
	"context"

	v1 "backend-service/api/evie/service/v1"
	"backend-service/app/evie/service/internal/conf"
	"backend-service/app/evie/service/internal/service"
	"backend-service/pkg/audit"
	"backend-service/pkg/auth"
	authzEngine "backend-service/pkg/auth/authz"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	"github.com/go-kratos/kratos/v2/transport/grpc"
)

func newGRPCWhiteListMatcher() selector.MatchFunc {
	whiteList := map[string]bool{
		// TODO: 添加白名单操作
	}
	return func(_ context.Context, operation string) bool {
		return !whiteList[operation]
	}
}

func NewGRPCServer(
	c *conf.Server,
	dictionaryService *service.DictionaryServiceService,
	hotwordService *service.HotwordServiceService,
	asrService *service.ASRServiceService,
	providerService *service.ProviderServiceService,
	correctionService *service.CorrectionServiceService,
	authenticator *auth.AuthToken,
	authorizer authzEngine.Authorizer,
	auditClient audit.Client,
	logger log.Logger,
) (*grpc.Server, error) {
	if c == nil || c.Grpc == nil {
		return nil, nil
	}
	middlewares, err := newServerMiddleware(c.Grpc.Middleware, newGRPCWhiteListMatcher(), logger, authenticator, authorizer, auditClient)
	if err != nil {
		return nil, err
	}
	srv := grpc.NewServer(
		grpc.Middleware(middlewares...),
		grpc.Address(c.Grpc.Addr),
	)
	v1.RegisterDictionaryServiceServer(srv, dictionaryService)
	v1.RegisterHotwordServiceServer(srv, hotwordService)
	v1.RegisterASRServiceServer(srv, asrService)
	v1.RegisterProviderServiceServer(srv, providerService)
	v1.RegisterCorrectionServiceServer(srv, correctionService)
	return srv, nil
}
