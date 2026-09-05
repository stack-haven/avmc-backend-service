// Package qua provides a qua-system-specific vocabulary Source
// adapter. It is implemented as a thin wrapper around
// evie/tool/pkg/source/http so that the generic HTTP adapter covers
// all qua-specific quirks (envelope shape, headers, token propagation).
//
// # Why this exists
//
// qua has a stable but quirky protocol:
//
//   - GET {baseURL}/admin-api/qua/member-extended/page?selectAll=true
//     returns {"code":0,"msg":"ok","data":{"list":[...],"total":N,...}}
//   - GET {baseURL}/admin-api/system/dept/list
//     returns {"code":0,"msg":"ok","data":[...]}
//   - The bearer token must be propagated on every call so the
//     upstream records per-user audit context.
//
// New deployments should configure the generic http adapter instead
// and skip this package; qua is preserved here as a working reference
// for the existing backend-service evie/tool users.
package qua

import (
	"context"

	"backend-service/app/evie/tool/pkg/source"
	httpsrc "backend-service/app/evie/tool/pkg/source/http"
)

// Entity type names emitted by qua (matched by the Normalizer rules).
const (
	UserEntityType       = "user"
	DepartmentEntityType = "department"
)

// AuthTokenProvider returns the caller's bearer token so qua can
// record per-user audit context.
type AuthTokenProvider interface {
	Token(ctx context.Context) (string, error)
}

// TokenFunc adapts a closure to AuthTokenProvider.
type TokenFunc func(ctx context.Context) (string, error)

// Token implements AuthTokenProvider.
func (f TokenFunc) Token(ctx context.Context) (string, error) { return f(ctx) }

// Config configures the qua Source. All fields are forwarded to the
// underlying http.Source; this struct simply fixes the conventions
// documented in the qua protocol notes.
type Config struct {
	BaseURL  string
	UserPath string
	DeptPath string

	// Tokens are propagated on every request (qua records per-user
	// audit). When Tokens is nil, no Authorization header is sent.
	Tokens AuthTokenProvider

	// Headers are sent on every request, in addition to Authorization.
	// Useful for static metadata (e.g. zone).
	Headers map[string]string

	// TenantHeader / TenantID, when set, send the tenant id under
	// the configured header name.
	TenantHeader string
	TenantID     string
}

// Source is a qua-system vocabulary adapter.
type Source struct {
	inner *httpsrc.Source
}

// New constructs a qua Source.
func New(cfg Config) (*Source, error) {
	hs, err := httpsrc.New(httpsrc.Config{
		BaseURL:        cfg.BaseURL,
		UserPath:       cfg.UserPath,
		DeptPath:       cfg.DeptPath,
		UserEntityType: UserEntityType,
		DeptEntityType: DepartmentEntityType,
		TokenProvider:  tokensToHTTP(cfg.Tokens),
		Headers:        cfg.Headers,
		TenantHeader:   cfg.TenantHeader,
		TenantID:       cfg.TenantID,
		Envelope: httpsrc.Envelope{
			UsersPath: "data.list",
			DeptsPath: "data",
			CodePath:  "code",
			CodeOK:    0,
		},
	})
	if err != nil {
		return nil, err
	}
	return &Source{inner: hs}, nil
}

// Name implements source.Source.
func (s *Source) Name() string { return "qua" }

// Fetch implements source.Source by delegating to the http adapter.
func (s *Source) Fetch(ctx context.Context) ([]source.RawEntity, error) {
	return s.inner.Fetch(ctx)
}

func tokensToHTTP(t AuthTokenProvider) httpsrc.AuthTokenProvider {
	if t == nil {
		return nil
	}
	return httpsrc.TokenFunc(t.Token)
}
