// Package static provides an in-memory credential.Provider primarily
// intended for local development, demos, integration tests, and
// single-tenant deployments where no external identity store is
// available.
//
// # Use case
//
// When the tool runs in standalone / demo mode (no business-system
// Redis, no JWT keypair) the operator declares a small list of
// (token, identity) pairs in the configuration file. StaticProvider
// matches incoming Bearer tokens against this list.
//
// # Example
//
//	p, _ := static.New(static.Config{
//	    Users: []static.User{
//	        {Token: "demo-token", TenantID: "demo", UserID: "u1", UserName: "Demo User"},
//	        {Token: "admin-token", TenantID: "demo", UserID: "admin", UserType: 1},
//	    },
//	})
package static

import (
	"context"
	"strings"
	"sync"
	"time"

	"backend-service/pkg/credential"
)

// User is a single (token, identity) pair.
type User struct {
	// Token is the literal Bearer token that the caller presents.
	Token string

	// TenantID, UserID, etc. are mapped directly onto CallerIdentity.
	// All fields except Token are optional.
	TenantID  string
	UserID    string
	UserName  string
	DeptID    string
	UserType  int32
	Scopes    []string
	ExpiresAt time.Time
}

// Config configures a StaticProvider.
type Config struct {
	// Users is the list of accepted (token, identity) pairs.
	Users []User

	// DefaultTenant, when non-empty, is used as the TenantID for any
	// token that matches a User entry without an explicit TenantID.
	DefaultTenant string
}

// Provider is an in-memory credential.Provider.
type Provider struct {
	mu             sync.RWMutex
	byTokenLower   map[string]credential.CallerIdentity
	defaultTenant  string
	caseSensitive  bool
}

// New constructs a StaticProvider from the given configuration.
//
// Tokens are matched case-insensitively to be tolerant of clients that
// normalize header values; use WithCaseSensitive if you need strict
// matching.
func New(cfg Config) (*Provider, error) {
	if len(cfg.Users) == 0 {
		return nil, credential.ErrInvalidConfig
	}
	p := &Provider{
		byTokenLower:  make(map[string]credential.CallerIdentity, len(cfg.Users)),
		defaultTenant: cfg.DefaultTenant,
	}
	for _, u := range cfg.Users {
		if u.Token == "" {
			continue
		}
		tenant := u.TenantID
		if tenant == "" {
			tenant = cfg.DefaultTenant
		}
		p.byTokenLower[strings.ToLower(u.Token)] = credential.CallerIdentity{
			TenantID:    tenant,
			UserID:      u.UserID,
			UserName:    u.UserName,
			DeptID:      u.DeptID,
			UserType:    u.UserType,
			Scopes:      u.Scopes,
			AccessToken: u.Token,
			ExpiresAt:   u.ExpiresAt,
		}
	}
	if len(p.byTokenLower) == 0 {
		return nil, credential.ErrInvalidConfig
	}
	return p, nil
}

// Name implements credential.Provider.
func (p *Provider) Name() string { return "static" }

// Authenticate looks up the token in the in-memory table.
func (p *Provider) Authenticate(_ context.Context, token string) (*credential.CallerIdentity, error) {
	if token == "" {
		return nil, credential.ErrTokenNotFound
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	id, ok := p.byTokenLower[strings.ToLower(token)]
	if !ok {
		return nil, credential.ErrTokenNotFound
	}
	return &id, nil
}

// AddUser dynamically inserts a User entry. Useful for tests; not
// thread-safe with respect to concurrent New/Add.
func (p *Provider) AddUser(u User) {
	if u.Token == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	tenant := u.TenantID
	if tenant == "" {
		tenant = p.defaultTenant
	}
	p.byTokenLower[strings.ToLower(u.Token)] = credential.CallerIdentity{
		TenantID:    tenant,
		UserID:      u.UserID,
		UserName:    u.UserName,
		DeptID:      u.DeptID,
		UserType:    u.UserType,
		Scopes:      u.Scopes,
		AccessToken: u.Token,
		ExpiresAt:   u.ExpiresAt,
	}
}
