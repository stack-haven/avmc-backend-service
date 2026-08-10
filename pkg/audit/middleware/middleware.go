package middleware

import (
	"context"
	"net"
	"strings"
	"time"

	"backend-service/pkg/audit"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	khttp "github.com/go-kratos/kratos/v2/transport/http"

	"github.com/google/uuid"
)

// Server returns a middleware that records non-read HTTP requests.
// Read operations (GET/HEAD/OPTIONS) are skipped.
func Server(client audit.Client, extractor audit.ContextExtractor, logger log.Logger) middleware.Middleware {
	h := log.NewHelper(log.With(logger, "module", "audit"))
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			start := time.Now()
			resp, err := handler(ctx, req)

			tr, ok := transport.FromServerContext(ctx)
			if !ok {
				return resp, err
			}
			if shouldSkip(tr) {
				return resp, err
			}

			record := buildRecord(ctx, tr, extractor, err, time.Since(start))
			if record != nil {
				go func() {
					if auditErr := client.Append(context.Background(), record); auditErr != nil {
						h.Warnf("audit append failed: %v", auditErr)
					}
				}()
			}

			return resp, err
		}
	}
}

func shouldSkip(tr transport.Transporter) bool {
	if tr.Kind() == transport.KindHTTP {
		if ht, ok := tr.(khttp.Transporter); ok && ht.Request() != nil {
			m := strings.ToUpper(ht.Request().Method)
			return m == "GET" || m == "HEAD" || m == "OPTIONS"
		}
	}
	return false
}

func buildRecord(
	ctx context.Context,
	tr transport.Transporter,
	extractor audit.ContextExtractor,
	handlerErr error,
	duration time.Duration,
) *audit.Record {
	record := &audit.Record{
		Module:     moduleFromPath(tr.Operation()),
		Action:     tr.Operation(),
		Method:     methodFromTransporter(tr),
		Path:       tr.Operation(),
		DurationMs: duration.Milliseconds(),
		Success:    handlerErr == nil,
	}

	if user := extractor(ctx); user.TenantID > 0 {
		record.TenantID = user.TenantID
		record.OperatorID = user.UserID
		record.OperatorName = user.UserName
	}

	if ip := clientIP(ctx, tr); ip != "" {
		record.IP = ip
	}
	record.TraceID = traceID(ctx)

	if handlerErr != nil {
		record.ErrorMessage = handlerErr.Error()
	}

	return record
}

func moduleFromPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 2 {
		return parts[0] + "." + parts[1]
	}
	return path
}

func methodFromTransporter(tr transport.Transporter) string {
	if ht, ok := tr.(khttp.Transporter); ok && ht.Request() != nil {
		return ht.Request().Method
	}
	return ""
}

func clientIP(ctx context.Context, tr transport.Transporter) string {
	if ht, ok := tr.(khttp.Transporter); ok && ht.Request() != nil {
		if fwd := ht.Request().Header.Get("X-Forwarded-For"); fwd != "" {
			return strings.Split(fwd, ",")[0]
		}
		host, _, _ := net.SplitHostPort(ht.Request().RemoteAddr)
		return host
	}
	return ""
}

func traceID(ctx context.Context) string {
	if id, ok := ctx.Value("trace_id").(string); ok {
		return id
	}
	return uuid.New().String()
}
