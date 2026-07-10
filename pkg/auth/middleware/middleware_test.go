package middleware

import (
	"context"
	nethttp "net/http"
	"testing"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/transport"
	"google.golang.org/grpc/metadata"

	"backend-service/pkg/auth/authn"
	"backend-service/pkg/auth/authz"
)

func TestAuthnMiddlewareRejectsIncompleteAuthentication(t *testing.T) {
	tests := []struct {
		name   string
		claims *authn.AuthClaims
		opts   authn.Options
	}{
		{
			name: "nil claims",
			opts: authn.Options{
				UserFactory: func(*authn.AuthClaims) authn.SecurityUser { return &testSecurityUser{} },
			},
		},
		{
			name:   "nil user factory",
			claims: testClaims(),
			opts:   authn.Options{},
		},
		{
			name:   "nil security user",
			claims: testClaims(),
			opts: authn.Options{
				UserFactory: func(*authn.AuthClaims) authn.SecurityUser { return nil },
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authenticator := &testAuthenticator{claims: tt.claims, opts: tt.opts}
			called := false
			handler := AuthnMiddleware(authenticator)(func(context.Context, interface{}) (interface{}, error) {
				called = true
				return nil, nil
			})

			_, err := handler(testGRPCContext(), nil)
			if err == nil {
				t.Fatal("expected authentication error")
			}
			if called {
				t.Fatal("handler must not be called")
			}
		})
	}
}

func TestAuthnMiddlewareInjectsClaimsAndUser(t *testing.T) {
	claims := testClaims()
	authenticator := &testAuthenticator{
		claims: claims,
		opts: authn.Options{
			UserFactory: func(*authn.AuthClaims) authn.SecurityUser {
				return &testSecurityUser{subject: "7", tenant: "3"}
			},
		},
	}
	handler := AuthnMiddleware(authenticator)(func(ctx context.Context, _ interface{}) (interface{}, error) {
		gotClaims, ok := authn.AuthClaimsFromContext(ctx)
		if !ok || gotClaims.GetSubject() != "7" {
			t.Fatal("claims were not injected")
		}
		gotUser, ok := authn.AuthUserFromContext(ctx)
		if !ok || gotUser.GetSubject() != "7" || gotUser.GetTenant() != "3" {
			t.Fatal("security user was not injected")
		}
		return "ok", nil
	})

	got, err := handler(testGRPCContext(), nil)
	if err != nil {
		t.Fatalf("authn middleware: %v", err)
	}
	if got != "ok" {
		t.Fatalf("response = %v", got)
	}
}

func TestAuthzMiddlewareUsesHTTPMethodAndTokenTenant(t *testing.T) {
	authorizer := &testAuthorizer{allowed: true}
	handler := AuthzMiddleware(authorizer)(func(ctx context.Context, _ interface{}) (interface{}, error) {
		allowed, ok := authz.AuthzResultFromContext(ctx)
		if !ok || !allowed {
			t.Fatal("authorization result was not injected")
		}
		return "ok", nil
	})

	ctx := authn.ContextWithAuthClaims(testHTTPContext(nethttp.MethodPut), testClaims())
	got, err := handler(ctx, nil)
	if err != nil {
		t.Fatalf("authz middleware: %v", err)
	}
	if got != "ok" {
		t.Fatalf("response = %v", got)
	}
	if authorizer.action != authz.Action(nethttp.MethodPut) {
		t.Fatalf("action = %q", authorizer.action)
	}
	if authorizer.tenant != "3" {
		t.Fatalf("tenant = %q", authorizer.tenant)
	}
}

func TestAuthzMiddlewareUsesGRPCMethodSegment(t *testing.T) {
	authorizer := &testAuthorizer{allowed: true}
	handler := AuthzMiddleware(authorizer)(func(context.Context, interface{}) (interface{}, error) {
		return "ok", nil
	})

	ctx := authn.ContextWithAuthClaims(testGRPCContext(), testClaims())
	got, err := handler(ctx, nil)
	if err != nil {
		t.Fatalf("authz middleware: %v", err)
	}
	if got != "ok" {
		t.Fatalf("response = %v", got)
	}
	if authorizer.action != "GetUser" || authorizer.object != "/admin.v1.UserService/GetUser" || authorizer.tenant != "3" {
		t.Fatalf("enforce object/action/tenant = %q/%q/%q", authorizer.object, authorizer.action, authorizer.tenant)
	}
}

func TestAuthzMiddlewareRejectsTransportWithoutClaims(t *testing.T) {
	handler := AuthzMiddleware(&testAuthorizer{allowed: true})(func(context.Context, interface{}) (interface{}, error) {
		t.Fatal("handler must not be called")
		return nil, nil
	})

	if _, err := handler(testGRPCContext(), nil); err != ErrInvalidToken {
		t.Fatalf("error = %v, want invalid token", err)
	}
}

func TestCombinedAuthMiddlewareInjectsUserAndAuthorizes(t *testing.T) {
	claims := testClaims()
	authenticator := &testAuthenticator{
		claims: claims,
		opts: authn.Options{
			UserFactory: func(*authn.AuthClaims) authn.SecurityUser {
				return &testSecurityUser{subject: "7", tenant: "3"}
			},
		},
	}
	authorizer := &testAuthorizer{allowed: true}
	handler := CombinedAuthMiddleware(authenticator, authorizer)(func(ctx context.Context, _ interface{}) (interface{}, error) {
		user, ok := authn.AuthUserFromContext(ctx)
		if !ok || user.GetSubject() != "7" {
			t.Fatal("security user was not injected")
		}
		allowed, ok := authz.AuthzResultFromContext(ctx)
		if !ok || !allowed {
			t.Fatal("authorization result was not injected")
		}
		return nil, nil
	})

	if _, err := handler(testHTTPContext(nethttp.MethodPost), nil); err != nil {
		t.Fatalf("combined middleware: %v", err)
	}
	if authorizer.action != authz.Action(nethttp.MethodPost) || authorizer.tenant != "3" {
		t.Fatalf("enforce action/tenant = %q/%q", authorizer.action, authorizer.tenant)
	}
}

func TestAuthnMiddlewareMapsAuthenticatorErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want error
	}{
		{name: "missing", err: authn.NewAuthError(authn.ErrCodeMissingToken, "missing", nil), want: ErrMissingToken},
		{name: "expired", err: authn.NewAuthError(authn.ErrCodeExpiredToken, "expired", nil), want: ErrExpiredToken},
		{name: "invalid", err: authn.NewAuthError(authn.ErrCodeInvalidToken, "invalid", nil), want: ErrInvalidToken},
		{name: "unknown", err: authn.NewAuthError(authn.ErrCodeUnknown, "other", nil), want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authenticator := &testAuthenticator{err: tt.err}
			handler := AuthnMiddleware(authenticator)(func(context.Context, interface{}) (interface{}, error) {
				t.Fatal("handler must not be called")
				return nil, nil
			})
			_, err := handler(context.Background(), nil)
			if tt.want != nil {
				if err != tt.want {
					t.Fatalf("error = %v, want %v", err, tt.want)
				}
				return
			}
			if err == nil || !errors.IsUnauthorized(err) {
				t.Fatalf("error = %v, want unauthorized", err)
			}
		})
	}
}

func TestAuthzMiddlewareFailsClosed(t *testing.T) {
	authorizer := &testAuthorizer{allowed: true}
	handler := AuthzMiddleware(authorizer)(func(context.Context, interface{}) (interface{}, error) {
		t.Fatal("handler must not be called")
		return nil, nil
	})

	if _, err := handler(context.Background(), nil); err != ErrPermissionDenied {
		t.Fatalf("error = %v, want permission denied", err)
	}
}

func TestDefaultGRPCAuthExtractor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		md      metadata.MD
		want    string
		wantErr bool
	}{
		{name: "bearer", md: metadata.Pairs("authorization", "Bearer abc"), want: "abc"},
		{name: "token scheme", md: metadata.Pairs("authorization", "Token abc"), want: "abc"},
		{name: "token header", md: metadata.Pairs("token", "abc"), want: "abc"},
		{name: "x token header", md: metadata.Pairs("x-token", "abc"), want: "abc"},
		{name: "missing", md: metadata.MD{}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := metadata.NewIncomingContext(context.Background(), tt.md)
			got, err := DefaultGRPCAuthExtractor(ctx)
			if (err != nil) != tt.wantErr {
				t.Fatalf("DefaultGRPCAuthExtractor() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("token = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDefaultGRPCAuthzInfoExtractor(t *testing.T) {
	t.Parallel()

	sub, obj, act, tenant, err := DefaultGRPCAuthzInfoExtractor(
		authn.ContextWithAuthClaims(context.Background(), testClaims()),
		"/admin.v1.UserService/GetUser",
	)
	if err != nil {
		t.Fatalf("DefaultGRPCAuthzInfoExtractor() error = %v", err)
	}
	if sub != "7" || tenant != "3" || obj != "/admin.v1.UserService/GetUser" || act != "GetUser" {
		t.Fatalf("authz info = sub:%q obj:%q act:%q tenant:%q", sub, obj, act, tenant)
	}

	if _, _, _, _, err := DefaultGRPCAuthzInfoExtractor(context.Background(), "/admin.v1.UserService/GetUser"); err == nil {
		t.Fatal("extractor without claims succeeded")
	}
}

func TestSkipAuthRoleMiddlewareUsesHTTPMethodAndFailsClosed(t *testing.T) {
	authorizer := &testAuthorizer{allowed: true}
	handler := SkipAuthRoleMiddleware(authorizer, []string{"super_admin"})(func(ctx context.Context, _ interface{}) (interface{}, error) {
		allowed, ok := authz.AuthzResultFromContext(ctx)
		if !ok || !allowed {
			t.Fatal("authorization result was not injected")
		}
		return "ok", nil
	})

	if _, err := handler(context.Background(), nil); err != ErrPermissionDenied {
		t.Fatalf("missing authz error = %v, want permission denied", err)
	}

	ctx := authn.ContextWithAuthClaims(testHTTPContext(nethttp.MethodDelete), testClaims())
	if _, err := handler(ctx, nil); err != nil {
		t.Fatalf("skip role middleware: %v", err)
	}
	if authorizer.action != authz.Action(nethttp.MethodDelete) {
		t.Fatalf("action = %q", authorizer.action)
	}
	if authorizer.tenant != "3" {
		t.Fatalf("tenant = %q", authorizer.tenant)
	}
}

func TestSkipAuthRoleMiddlewareInjectsResultWhenRoleSkips(t *testing.T) {
	authorizer := &testAuthorizer{roles: []authz.Subject{"super_admin"}}
	handler := SkipAuthRoleMiddleware(authorizer, []string{"super_admin"})(func(ctx context.Context, _ interface{}) (interface{}, error) {
		allowed, ok := authz.AuthzResultFromContext(ctx)
		if !ok || !allowed {
			t.Fatal("authorization result was not injected")
		}
		return nil, nil
	})

	ctx := authn.ContextWithAuthClaims(testHTTPContext(nethttp.MethodGet), testClaims())
	if _, err := handler(ctx, nil); err != nil {
		t.Fatalf("skip role middleware: %v", err)
	}
	if authorizer.enforced {
		t.Fatal("enforce must not be called for skipped role")
	}
}

type testAuthenticator struct {
	authn.Authenticator
	claims *authn.AuthClaims
	err    error
	opts   authn.Options
}

func (a *testAuthenticator) Authenticate(context.Context) (*authn.AuthClaims, error) {
	return a.claims, a.err
}

func (a *testAuthenticator) Options() authn.Options {
	return a.opts
}

type testSecurityUser struct {
	subject string
	tenant  string
}

func (u *testSecurityUser) Name() string                           { return "test" }
func (u *testSecurityUser) ParseFromContext(context.Context) error { return nil }
func (u *testSecurityUser) GetSubject() string                     { return u.subject }
func (u *testSecurityUser) GetObject() string                      { return "" }
func (u *testSecurityUser) GetAction() string                      { return "" }
func (u *testSecurityUser) GetTenant() string                      { return u.tenant }

type testAuthorizer struct {
	authz.Authorizer
	allowed  bool
	roles    []authz.Subject
	enforced bool
	subject  authz.Subject
	object   authz.Object
	action   authz.Action
	tenant   authz.Tenant
}

func (a *testAuthorizer) Enforce(_ context.Context, sub authz.Subject, obj authz.Object, act authz.Action, tenant authz.Tenant) (bool, error) {
	a.enforced = true
	a.subject = sub
	a.object = obj
	a.action = act
	a.tenant = tenant
	return a.allowed, nil
}

func (a *testAuthorizer) GetRolesForUser(context.Context, authz.Subject, authz.Tenant) ([]authz.Subject, error) {
	return a.roles, nil
}

type testTransport struct {
	kind      transport.Kind
	operation string
	request   *nethttp.Request
}

func (t *testTransport) Kind() transport.Kind            { return t.kind }
func (t *testTransport) Endpoint() string                { return "" }
func (t *testTransport) Operation() string               { return t.operation }
func (t *testTransport) RequestHeader() transport.Header { return nil }
func (t *testTransport) ReplyHeader() transport.Header   { return nil }
func (t *testTransport) Request() *nethttp.Request       { return t.request }
func (t *testTransport) PathTemplate() string            { return t.operation }

func testClaims() *authn.AuthClaims {
	claims := authn.AuthClaims{"sub": "7", "tenant": "3", "iss": "not-tenant"}
	return &claims
}

func testHTTPContext(method string) context.Context {
	return transport.NewServerContext(context.Background(), &testTransport{
		kind:      transport.KindHTTP,
		operation: "/admin.v1.UserService/UpdateUser",
		request:   &nethttp.Request{Method: method},
	})
}

func testGRPCContext() context.Context {
	return transport.NewServerContext(context.Background(), &testTransport{
		kind:      transport.KindGRPC,
		operation: "/admin.v1.UserService/GetUser",
	})
}
