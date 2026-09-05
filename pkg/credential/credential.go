// Package credential provides pluggable authentication for tools that
// need to verify Bearer tokens issued by external business systems.
//
// # Design goals
//
//   - Business-system agnostic: no hard-coded references to any specific
//     product, key prefix, or JSON schema.
//   - Pluggable: implement Provider to integrate any backing store
//     (Redis, JWT, static config, etc.).
//   - Zero-config demo: a built-in StaticProvider lets the tool run
//     without external dependencies for local development.
//   - Configurable JSON mapping: when reading tokens from external
//     stores (e.g. a shared Redis), every JSON field used to build a
//     CallerIdentity is named via configuration, not code.
//
// # Typical wiring
//
//	cfg := credential.Config{
//	    Provider: "static",
//	    Static: credential.StaticConfig{
//	        Users: []credential.StaticUser{{
//	            Token: "demo-token", TenantID: "demo", UserID: "u1",
//	        }},
//	    },
//	}
//	p, _ := credential.NewProvider(cfg, nil)
//	id, err := p.Authenticate(ctx, "demo-token")
//
// # Integration with HTTP/gRPC middleware
//
//	pkg/credential/middleware provides a ready-to-use middleware that
// extracts the Bearer token, calls Provider.Authenticate, and stores the
// resulting *CallerIdentity in the request context.
package credential

import (
	"context"
	"errors"
	"time"
)

// Common errors returned by Provider implementations.
var (
	// ErrTokenNotFound is returned when the token does not exist in the
	// backing store (expired, revoked, or never issued).
	ErrTokenNotFound = errors.New("credential: token not found")

	// ErrTokenInvalid is returned when the token exists but the payload
	// is malformed, expired, or fails signature verification.
	ErrTokenInvalid = errors.New("credential: token invalid")

	// ErrProviderUnavailable is returned when the backing store cannot
	// be reached (network, timeout, DNS, etc.).
	ErrProviderUnavailable = errors.New("credential: provider unavailable")

	// ErrInvalidConfig is returned when a Provider is constructed with
	// missing or contradictory configuration.
	ErrInvalidConfig = errors.New("credential: invalid config")
)

// CallerIdentity is the tool's view of an authenticated caller.
//
// All fields are strings (no numeric types) because external systems
// frequently use IDs that exceed 2^63 or contain leading zeros.
//
// Scopes is the raw value returned by the upstream system. Whether it
// is parsed into []string or kept as-is depends on the provider.
type CallerIdentity struct {
	// TenantID identifies the tenant (organization) the caller belongs
	// to. Required for multi-tenant routing.
	TenantID string

	// UserID identifies the caller within the tenant.
	UserID string

	// UserName is a human-readable display name (optional).
	UserName string

	// DeptID is the caller's department within the tenant (optional).
	DeptID string

	// UserType classifies the caller (e.g. 2 = regular user, 1 = admin).
	// Zero means "unknown"; providers may leave it unset.
	UserType int32

	// AccessToken is the original bearer token, preserved so downstream
	// calls can forward it (when the upstream system expects token
	// propagation rather than service-account impersonation).
	AccessToken string

	// RefreshToken is the upstream refresh token, if any.
	RefreshToken string

	// Scopes is the raw scope payload (string, []string, map, ...).
	// Providers MUST NOT mutate this; consumers decide how to parse it.
	Scopes any

	// ExpiresAt is when the upstream token expires. Zero means "unknown".
	ExpiresAt time.Time

	// Raw is the unparsed backing-store payload, available for callers
	// that need fields not covered by the canonical struct above.
	// Providers SHOULD populate this when the backing store returns
	// structured data (e.g. JSON object).
	Raw any
}

// Provider verifies a Bearer token and returns the corresponding
// CallerIdentity.
//
// Implementations MUST be safe for concurrent use by multiple goroutines.
type Provider interface {
	// Name returns the implementation name (e.g. "redis", "jwt",
	// "static"). Used for diagnostics and config validation.
	Name() string

	// Authenticate verifies the token and returns the caller's identity.
	//
	// Errors:
	//   - ErrTokenNotFound: token unknown / expired
	//   - ErrTokenInvalid:  payload malformed / signature failed
	//   - ErrProviderUnavailable: backing store unreachable
	//   - other wrapped errors: implementation-specific failures
	Authenticate(ctx context.Context, token string) (*CallerIdentity, error)
}

// FieldMapper describes how to extract CallerIdentity fields from a
// raw payload (e.g. a JSON object read from Redis).
//
// All Field* names are JSON keys (camelCase / snake_case as the upstream
// system uses). Leaving a field empty means "do not extract"; the
// corresponding CallerIdentity field stays at its zero value.
type FieldMapper struct {
	// TenantID is the JSON key for the tenant identifier (e.g. "tenantId").
	TenantID string

	// UserID is the JSON key for the user identifier (e.g. "userId").
	UserID string

	// UserName is the JSON key for the display name (e.g. "nickname").
	// May be a dotted path like "userInfo.nickname" for nested objects.
	UserName string

	// DeptID is the JSON key for the department identifier (optional).
	DeptID string

	// UserType is the JSON key for the user-type enum (optional).
	UserType string

	// AccessToken is the JSON key for the access token echo (optional).
	AccessToken string

	// RefreshToken is the JSON key for the refresh token (optional).
	RefreshToken string

	// Scopes is the JSON key for the scopes payload (optional).
	Scopes string

	// ExpiresAt is the JSON key for the expiry timestamp.
	// The value is parsed with ParseExpiresAt.
	ExpiresAt string
}
