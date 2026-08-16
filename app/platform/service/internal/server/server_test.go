package server

import (
	"context"
	"strings"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"

	v1 "backend-service/api/platform/service/v1"
	"backend-service/app/platform/service/internal/conf"
	"backend-service/pkg/auth/authn"
)

func TestNewServerMiddlewareRejectsUnsupportedLimiter(t *testing.T) {
	cfg := &conf.Middleware{
		Auth:    &conf.Middleware_Auth{},
		Limiter: &conf.Middleware_RateLimiter{Name: "token-bucket"},
	}

	_, err := newServerMiddleware(cfg, func(context.Context, string) bool { return true }, func(context.Context, string) bool { return true }, log.DefaultLogger, nil, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "unsupported rate limiter") {
		t.Fatalf("newServerMiddleware() error = %v", err)
	}
}

func TestNewServerMiddlewareAcceptsDisabledLimiter(t *testing.T) {
	for _, name := range []string{"", "off", "none", "disabled"} {
		t.Run(name, func(t *testing.T) {
			cfg := &conf.Middleware{
				EnableRecovery: true,
				Auth:           &conf.Middleware_Auth{},
				Limiter:        &conf.Middleware_RateLimiter{Name: name},
			}
			middlewares, err := newServerMiddleware(cfg, func(context.Context, string) bool { return true }, func(context.Context, string) bool { return true }, log.DefaultLogger, nil, nil, nil)
			if err != nil {
				t.Fatalf("newServerMiddleware() error = %v", err)
			}
			// recovery + Authn + Authz + platformControl = 4
			if len(middlewares) != 4 {
				t.Fatalf("middleware count = %d, want 4", len(middlewares))
			}
		})
	}
}

func TestPlatformControlServerAllowsNonPlatformOperations(t *testing.T) {
	m := platformControlServer()
	called := false
	handler := func(ctx context.Context, _ interface{}) (interface{}, error) {
		called = true
		return "ok", nil
	}
	// Wrap with a handler that has a transport context with a non-platform operation.
	wrapped := func(ctx context.Context, req interface{}) (interface{}, error) {
		return m(handler)(ctx, req)
	}
	// Empty context (no transport) — should pass through.
	resp, err := wrapped(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "ok" || !called {
		t.Fatalf("expected handler to be called, got resp=%v called=%v", resp, called)
	}
}

func TestPlatformControlServerRejectsNonOperatorOnPlatformOp(t *testing.T) {
	m := platformControlServer()
	called := false
	handler := func(ctx context.Context, _ interface{}) (interface{}, error) {
		called = true
		return "ok", nil
	}
	// Create a context with transport that has a platform control-plane operation
	// but no platform_operator claim.
	ctx := transport.NewServerContext(
		context.Background(),
		&mockTransporter{operation: v1.OperationTenantServiceCreateTenant},
	)
	// No auth claims set, so IsPlatformOperator returns false.
	_, err := m(handler)(ctx, nil)
	if err == nil {
		t.Fatal("expected platform operator required error, got nil")
	}
	if called {
		t.Fatal("handler should not be called when platform operator check fails")
	}
}

func TestPlatformControlServerAllowsPlatformOperator(t *testing.T) {
	m := platformControlServer()
	called := false
	handler := func(ctx context.Context, _ interface{}) (interface{}, error) {
		called = true
		return "ok", nil
	}
	// Create context with platform operator claim.
	claims := authn.AuthClaims{
		"sub":               "1",
		"tenant":            "0",
		"platform_operator": true,
	}
	ctx := authn.ContextWithAuthClaims(context.Background(), &claims)
	ctx = transport.NewServerContext(ctx, &mockTransporter{
		operation: v1.OperationTenantServiceCreateTenant,
	})
	resp, err := m(handler)(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != "ok" || !called {
		t.Fatalf("expected handler to be called, got resp=%v called=%v", resp, called)
	}
}

type mockTransporter struct {
	operation string
}

func (m *mockTransporter) Kind() transport.Kind            { return transport.KindGRPC }
func (m *mockTransporter) Endpoint() string                { return "" }
func (m *mockTransporter) Operation() string               { return m.operation }
func (m *mockTransporter) RequestHeader() transport.Header { return nil }
func (m *mockTransporter) ReplyHeader() transport.Header   { return nil }

var _ middleware.Middleware = platformControlServer()
