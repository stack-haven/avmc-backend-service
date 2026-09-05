// Package middleware provides HTTP and gRPC middleware that extract a
// Bearer token, verify it via credential.Provider, and inject the
// resulting *CallerIdentity into the request context.
//
// # HTTP
//
//	mw := middleware.HTTPMiddleware(middleware.Config{Provider: provider})
//	handler := mw(next)
//
// # gRPC
//
//	mw := middleware.GRPCUnaryInterceptor(middleware.Config{Provider: provider})
//	server := grpc.NewServer(grpc.UnaryInterceptor(mw))
//
// The downstream handler retrieves the identity with
// FromContext(ctx).
package middleware

import (
	"context"
	"net/http"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"backend-service/app/evie/tool/pkg/credential"
)

// ctxKey is unexported so callers cannot accidentally collide with it.
type ctxKey struct{}

// FromContext returns the verified CallerIdentity stored in ctx by
// the middleware, or (nil, false) when no identity is present.
func FromContext(ctx context.Context) (*credential.CallerIdentity, bool) {
	v := ctx.Value(ctxKey{})
	id, ok := v.(*credential.CallerIdentity)
	return id, ok
}

// WithIdentity returns a new context with id attached. Primarily used
// by tests to seed a context.
func WithIdentity(ctx context.Context, id *credential.CallerIdentity) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// Config configures both HTTP and gRPC middleware.
type Config struct {
	// Provider verifies the Bearer token. Required.
	Provider credential.Provider

	// HeaderName is the HTTP header carrying the token
	// (default: "Authorization").
	HeaderName string

	// Scheme is the expected scheme prefix (default: "Bearer ").
	Scheme string

	// SkipPaths is a list of HTTP paths that bypass authentication
	// (e.g. "/healthz"). Each entry is either an exact match or a
	// "/prefix*" wildcard suffix.
	SkipPaths []string

	// SkipOnMissing allows the request to proceed when no token is
	// present. Defaults to false (missing token → 401).
	SkipOnMissing bool

	// UnauthHandler is called when authentication fails. When nil, a
	// plain "401 Unauthorized" is returned.
	UnauthHandler func(w http.ResponseWriter, r *http.Request, reason string)
}

// HTTPMiddleware returns an http.Handler middleware.
func HTTPMiddleware(cfg Config) func(http.Handler) http.Handler {
	header := orDefault(cfg.HeaderName, "Authorization")
	scheme := orDefault(cfg.Scheme, "Bearer ")
	skip, wildcards := splitSkipPaths(cfg.SkipPaths)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if skip[r.URL.Path] || matchesWildcard(r.URL.Path, wildcards) {
				next.ServeHTTP(w, r)
				return
			}
			token, ok := extractToken(r.Header.Get(header), scheme)
			if !ok {
				if cfg.SkipOnMissing {
					next.ServeHTTP(w, r)
					return
				}
				writeUnauth(cfg, w, r, "missing token")
				return
			}
			id, err := cfg.Provider.Authenticate(r.Context(), token)
			if err != nil {
				writeUnauth(cfg, w, r, err.Error())
				return
			}
			ctx := WithIdentity(r.Context(), id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GRPCUnaryInterceptor returns a gRPC unary server interceptor.
func GRPCUnaryInterceptor(cfg Config) grpc.UnaryServerInterceptor {
	scheme := orDefault(cfg.Scheme, "Bearer ")
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			if cfg.SkipOnMissing {
				return handler(ctx, req)
			}
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}
		vals := md.Get("authorization")
		if len(vals) == 0 {
			if cfg.SkipOnMissing {
				return handler(ctx, req)
			}
			return nil, status.Error(codes.Unauthenticated, "missing authorization")
		}
		token, ok := extractToken(vals[0], scheme)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "bad authorization scheme")
		}
		id, err := cfg.Provider.Authenticate(ctx, token)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, err.Error())
		}
		ctx = WithIdentity(ctx, id)
		return handler(ctx, req)
	}
}

// --- helpers ---

func extractToken(value, scheme string) (string, bool) {
	if value == "" {
		return "", false
	}
	if scheme == "" {
		return value, true
	}
	if !strings.HasPrefix(value, scheme) {
		return "", false
	}
	return strings.TrimSpace(value[len(scheme):]), true
}

func splitSkipPaths(paths []string) (exact map[string]bool, wildcards []string) {
	exact = make(map[string]bool, len(paths))
	for _, p := range paths {
		if strings.HasSuffix(p, "*") {
			wildcards = append(wildcards, strings.TrimSuffix(p, "*"))
		} else {
			exact[p] = true
		}
	}
	return exact, wildcards
}

func matchesWildcard(path string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func writeUnauth(cfg Config, w http.ResponseWriter, r *http.Request, reason string) {
	if cfg.UnauthHandler != nil {
		cfg.UnauthHandler(w, r, reason)
		return
	}
	w.Header().Set("WWW-Authenticate", `Bearer realm="evie-tool"`)
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"code":401,"reason":"` + reason + `"}`))
}
