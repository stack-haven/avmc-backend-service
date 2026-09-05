package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc/metadata"

	"backend-service/app/evie/tool/pkg/credential"
	"backend-service/app/evie/tool/pkg/credential/middleware"
	"backend-service/app/evie/tool/pkg/credential/static"
)

func okHandler(checked *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := middleware.FromContext(r.Context())
		if !ok || id == nil {
			http.Error(w, "no identity", http.StatusInternalServerError)
			return
		}
		*checked = true
		w.WriteHeader(http.StatusOK)
	})
}

func TestHTTPMiddleware_AuthenticatesAndAttaches(t *testing.T) {
	prov, _ := static.New(static.Config{
		Users: []static.User{{Token: "tok", TenantID: "t1", UserID: "u1"}},
	})
	var gotIdentity bool
	h := middleware.HTTPMiddleware(middleware.Config{Provider: prov})(okHandler(&gotIdentity))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer tok")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if !gotIdentity {
		t.Errorf("expected handler to see identity in ctx")
	}
}

func TestHTTPMiddleware_MissingToken(t *testing.T) {
	prov, _ := static.New(static.Config{
		Users: []static.User{{Token: "tok"}},
	})
	h := middleware.HTTPMiddleware(middleware.Config{Provider: prov})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("handler should not run")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rr.Code)
	}
}

func TestHTTPMiddleware_BadScheme(t *testing.T) {
	prov, _ := static.New(static.Config{Users: []static.User{{Token: "tok"}}})
	h := middleware.HTTPMiddleware(middleware.Config{Provider: prov})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("handler should not run")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz") // wrong scheme
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rr.Code)
	}
}

func TestHTTPMiddleware_BadToken(t *testing.T) {
	prov, _ := static.New(static.Config{Users: []static.User{{Token: "tok"}}})
	h := middleware.HTTPMiddleware(middleware.Config{Provider: prov})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("handler should not run on bad token")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rr.Code)
	}
}

func TestHTTPMiddleware_SkipPaths(t *testing.T) {
	prov, _ := static.New(static.Config{Users: []static.User{{Token: "tok"}}})
	called := false
	h := middleware.HTTPMiddleware(middleware.Config{
		Provider: prov,
		SkipPaths: []string{"/healthz", "/api/public*"},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	for _, path := range []string{"/healthz", "/api/public/foo"} {
		req := httptest.NewRequest("GET", path, nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", path, rr.Code)
		}
		if !called {
			t.Errorf("%s: handler not called", path)
		}
	}
}

func TestHTTPMiddleware_SkipOnMissing(t *testing.T) {
	prov, _ := static.New(static.Config{Users: []static.User{{Token: "tok"}}})
	var anonymousReached bool
	h := middleware.HTTPMiddleware(middleware.Config{
		Provider:       prov,
		SkipOnMissing:  true,
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		anonymousReached = true
		if _, ok := middleware.FromContext(r.Context()); ok {
			t.Errorf("identity should not be present in anonymous mode")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if !anonymousReached {
		t.Errorf("handler should run when SkipOnMissing")
	}
}

func TestHTTPMiddleware_CustomUnauthHandler(t *testing.T) {
	prov, _ := static.New(static.Config{Users: []static.User{{Token: "tok"}}})
	called := false
	h := middleware.HTTPMiddleware(middleware.Config{
		Provider: prov,
		UnauthHandler: func(w http.ResponseWriter, r *http.Request, reason string) {
			called = true
			if reason == "" {
				t.Errorf("reason should be populated")
			}
			w.WriteHeader(http.StatusTeapot) // arbitrary
		},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("handler should not run")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusTeapot {
		t.Errorf("status = %d, want 418", rr.Code)
	}
	if !called {
		t.Errorf("UnauthHandler not called")
	}
}

func TestGRPCMiddleware_AuthenticatesAndAttaches(t *testing.T) {
	prov, _ := static.New(static.Config{
		Users: []static.User{{Token: "tok", TenantID: "t1", UserID: "u1"}},
	})
	interceptor := middleware.GRPCUnaryInterceptor(middleware.Config{Provider: prov})

	handler := func(ctx context.Context, req any) (any, error) {
		id, ok := middleware.FromContext(ctx)
		if !ok || id == nil {
			t.Errorf("identity missing from ctx")
		}
		return "ok", nil
	}

	// Build a ctx with authorization metadata.
	md := newMetadata("authorization", "Bearer tok")
	ctx := newCtxWithMetadata(md)

	out, err := interceptor(ctx, "req", nil, handler)
	if err != nil {
		t.Fatalf("interceptor: %v", err)
	}
	if out != "ok" {
		t.Errorf("out = %v, want ok", out)
	}
}

func TestGRPCMiddleware_MissingAuth(t *testing.T) {
	prov, _ := static.New(static.Config{Users: []static.User{{Token: "tok"}}})
	interceptor := middleware.GRPCUnaryInterceptor(middleware.Config{Provider: prov})

	handler := func(ctx context.Context, req any) (any, error) {
		t.Errorf("handler should not run")
		return nil, nil
	}

	ctx := newCtxWithMetadata(newMetadata())
	_, err := interceptor(ctx, "req", nil, handler)
	if err == nil {
		t.Errorf("expected Unauthenticated error")
	}
}

func TestGRPCMiddleware_BadToken(t *testing.T) {
	prov, _ := static.New(static.Config{Users: []static.User{{Token: "tok"}}})
	interceptor := middleware.GRPCUnaryInterceptor(middleware.Config{Provider: prov})

	handler := func(ctx context.Context, req any) (any, error) {
		t.Errorf("handler should not run")
		return nil, nil
	}

	md := newMetadata("authorization", "Bearer wrong")
	ctx := newCtxWithMetadata(md)
	_, err := interceptor(ctx, "req", nil, handler)
	if err == nil || !errors.Is(err, err) { // just check non-nil
		t.Errorf("expected error, got %v", err)
	}
}

// Helper imports are wrapped to keep this file self-contained.
var _ = credential.CallerIdentity{}

// --- helpers ---

func newMetadata(kv ...string) metadata.MD {
	if len(kv) == 0 {
		return metadata.MD{}
	}
	md := metadata.MD{}
	for i := 0; i+1 < len(kv); i += 2 {
		md[kv[i]] = []string{kv[i+1]}
	}
	return md
}

func newCtxWithMetadata(md metadata.MD) context.Context {
	return metadata.NewIncomingContext(context.Background(), md)
}
