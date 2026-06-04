package safelogging

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"github.com/go-kratos/kratos/v2/transport/http/status"
	"google.golang.org/grpc/codes"
)

// Server logs request metadata without serializing request bodies or credentials.
func Server(logger log.Logger) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (reply any, err error) {
			start := time.Now()
			reply, err = handler(ctx, req)

			code := int32(status.FromGRPCCode(codes.OK))
			reason := ""
			if se := errors.FromError(err); se != nil {
				code = se.Code
				reason = se.Reason
			}
			kind, operation := "", ""
			if info, ok := transport.FromServerContext(ctx); ok {
				kind = info.Kind().String()
				operation = info.Operation()
			}
			fields := []any{
				"kind", "server",
				"component", kind,
				"operation", operation,
				"code", code,
				"reason", reason,
				"latency", time.Since(start).Seconds(),
			}
			level := log.LevelInfo
			if code >= 500 {
				level = log.LevelError
				fields = append(fields, "error", err)
			} else if code >= 400 {
				level = log.LevelWarn
			}
			log.NewHelper(log.WithContext(ctx, logger)).Log(level, fields...)
			return reply, err
		}
	}
}
