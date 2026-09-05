// Package redis provides a credential.Provider backed by a Redis store.
//
// # Use case
//
// Many business systems centralise Bearer tokens in Redis with a key
// like "<prefix>:<token>" and a JSON value describing the session.
// RedisProvider reads that JSON value and projects it into a
// credential.CallerIdentity using a configurable FieldMapper.
//
// # Example
//
//	import "backend-service/app/evie/tool/pkg/credential/redis"
//
//	p, err := redis.New(redis.Config{
//	    Client: rdb,
//	    KeyPrefix: "oauth2_access_token:",
//	    Fields: redis.FieldMapper{
//	        TenantID:    "tenantId",
//	        UserID:      "userId",
//	        UserName:    "userInfo.nickname",
//	        UserType:    "userType",
//	        ExpiresAt:   "expiresTime",
//	        AccessToken: "accessToken",
//	    },
//	})
package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"

	"backend-service/app/evie/tool/pkg/credential"
)

// FieldMapper is an alias for credential.FieldMapper so callers can
// reference a single struct under either name.
type FieldMapper = credential.FieldMapper

// Config configures RedisProvider.
//
// All field names (TenantID, UserID, ...) are JSON keys used to extract
// the corresponding CallerIdentity field. Leaving one empty means "do
// not extract"; the corresponding CallerIdentity field stays at its
// zero value.
type Config struct {
	// Client is a connected go-redis client.
	Client *redis.Client

	// KeyPrefix is prepended to the bearer token when building the
	// Redis key (default: "oauth2_access_token:" if empty).
	KeyPrefix string

	// Fields describes how to project the JSON payload into a
	// CallerIdentity.
	Fields FieldMapper
}

// Provider implements credential.Provider using Redis as the backing store.
type Provider struct {
	client *redis.Client
	prefix string
	fields FieldMapper
}

// New constructs a RedisProvider.
func New(cfg Config) (*Provider, error) {
	if cfg.Client == nil {
		return nil, fmt.Errorf("%w: redis client is nil", credential.ErrInvalidConfig)
	}
	prefix := cfg.KeyPrefix
	if prefix == "" {
		prefix = "oauth2_access_token:"
	}
	return &Provider{
		client: cfg.Client,
		prefix: prefix,
		fields: cfg.Fields,
	}, nil
}

// Name implements credential.Provider.
func (p *Provider) Name() string { return "redis" }

// Key returns the Redis key for a given bearer token. Exposed for tests.
func (p *Provider) Key(token string) string { return p.prefix + token }

// Authenticate fetches the token payload from Redis and projects it
// into a CallerIdentity.
func (p *Provider) Authenticate(ctx context.Context, token string) (*credential.CallerIdentity, error) {
	if token == "" {
		return nil, fmt.Errorf("%w: empty token", credential.ErrTokenNotFound)
	}
	raw, err := p.client.Get(ctx, p.Key(token)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, credential.ErrTokenNotFound
		}
		return nil, fmt.Errorf("%w: redis GET failed: %v", credential.ErrProviderUnavailable, err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, fmt.Errorf("%w: %v", credential.ErrTokenInvalid, err)
	}
	id := credential.MapFromMapper(payload, p.fields)
	id.AccessToken = token // ensure the bearer itself is always present
	if id.AccessToken == "" {
		// payload may carry its own accessToken echo; preserve it when set.
		id.AccessToken = credential.ExtractString(credential.LookupPath(payload, p.fields.AccessToken))
	}
	return &id, nil
}
