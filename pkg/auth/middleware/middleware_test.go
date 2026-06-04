package middleware

import (
	"context"
	nethttp "net/http"
	"testing"

	"github.com/go-kratos/kratos/v2/transport"

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
				return &testSecurityUser{subject: "7", domain: "3"}
			},
		},
	}
	handler := AuthnMiddleware(authenticator)(func(ctx context.Context, _ interface{}) (interface{}, error) {
		gotClaims, ok := authn.AuthClaimsFromContext(ctx)
		if !ok || gotClaims.GetSubject() != "7" {
			t.Fatal("claims were not injected")
		}
		gotUser, ok := authn.AuthUserFromContext(ctx)
		if !ok || gotUser.GetSubject() != "7" || gotUser.GetDomain() != "3" {
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

func TestAuthzMiddlewareUsesHTTPMethodAndTokenDomain(t *testing.T) {
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
	if authorizer.domain != "3" {
		t.Fatalf("domain = %q", authorizer.domain)
	}
}

func TestCombinedAuthMiddlewareInjectsUserAndAuthorizes(t *testing.T) {
	claims := testClaims()
	authenticator := &testAuthenticator{
		claims: claims,
		opts: authn.Options{
			UserFactory: func(*authn.AuthClaims) authn.SecurityUser {
				return &testSecurityUser{subject: "7", domain: "3"}
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
	if authorizer.action != authz.Action(nethttp.MethodPost) || authorizer.domain != "3" {
		t.Fatalf("enforce action/domain = %q/%q", authorizer.action, authorizer.domain)
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
	if authorizer.domain != "3" {
		t.Fatalf("domain = %q", authorizer.domain)
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
	domain  string
}

func (u *testSecurityUser) Name() string                           { return "test" }
func (u *testSecurityUser) ParseFromContext(context.Context) error { return nil }
func (u *testSecurityUser) GetSubject() string                     { return u.subject }
func (u *testSecurityUser) GetObject() string                      { return "" }
func (u *testSecurityUser) GetAction() string                      { return "" }
func (u *testSecurityUser) GetDomain() string                      { return u.domain }

type testAuthorizer struct {
	authz.Authorizer
	allowed  bool
	roles    []authz.Subject
	enforced bool
	subject  authz.Subject
	object   authz.Object
	action   authz.Action
	domain   authz.Domain
}

func (a *testAuthorizer) Enforce(_ context.Context, sub authz.Subject, obj authz.Object, act authz.Action, dom authz.Domain) (bool, error) {
	a.enforced = true
	a.subject = sub
	a.object = obj
	a.action = act
	a.domain = dom
	return a.allowed, nil
}

func (a *testAuthorizer) GetRolesForUser(context.Context, authz.Subject, authz.Domain) ([]authz.Subject, error) {
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
	claims := authn.AuthClaims{"sub": "7", "dom": "3", "iss": "not-domain"}
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
