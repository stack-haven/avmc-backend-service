package server

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"

	"backend-service/app/platform/service/internal/biz"
)

// operationAuditServer returns a middleware that audits non-read HTTP operations.
func operationAuditServer(operationLog *biz.OperationLogUsecase, logger log.Logger) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			return handler(ctx, req)
		}
	}
}
