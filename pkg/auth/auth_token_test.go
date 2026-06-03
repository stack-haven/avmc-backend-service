package auth

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"

	"backend-service/pkg/auth/authn"
	authnJwt "backend-service/pkg/auth/authn/jwt"
)

func TestAuthTokenValidatesOnlyCurrentStoredToken(t *testing.T) {
	ctx := context.Background()
	token := newTestAuthToken(t)

	access, refresh, err := token.GenerateToken(ctx, AuthTokenInfo{
		UserId:   42,
		Username: "tester",
		DomainId: 1,
	})
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	claims, err := token.ValidateToken(ctx, access)
	if err != nil {
		t.Fatalf("validate access token: %v", err)
	}
	if got := claims.GetSubject(); got != "42" {
		t.Fatalf("subject = %q", got)
	}
	if _, err := token.ValidateRefreshToken(ctx, refresh); err != nil {
		t.Fatalf("validate refresh token: %v", err)
	}

	replacement, err := token.GenerateAccessToken(ctx, AuthTokenInfo{
		UserId:   42,
		Username: "tester",
		DomainId: 1,
	})
	if err != nil {
		t.Fatalf("generate replacement access token: %v", err)
	}
	if replacement == access {
		t.Fatal("replacement access token should differ")
	}
	if _, err := token.ValidateToken(ctx, access); err == nil {
		t.Fatal("old access token should be rejected after replacement")
	}
	if _, err := token.ValidateToken(ctx, replacement); err != nil {
		t.Fatalf("replacement access token should validate: %v", err)
	}
}

func TestAuthTokenRemoveRevokesAccessAndRefreshTokens(t *testing.T) {
	ctx := context.Background()
	token := newTestAuthToken(t)

	access, refresh, err := token.GenerateToken(ctx, AuthTokenInfo{
		UserId:   7,
		Username: "tester",
		DomainId: 2,
	})
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	if !token.IsExistAccessToken(ctx, 7) {
		t.Fatal("expected access token to exist")
	}
	if !token.IsExistRefreshToken(ctx, 7) {
		t.Fatal("expected refresh token to exist")
	}

	if err := token.RemoveToken(ctx, 7); err != nil {
		t.Fatalf("remove token: %v", err)
	}
	if token.IsExistAccessToken(ctx, 7) {
		t.Fatal("expected access token to be removed")
	}
	if token.IsExistRefreshToken(ctx, 7) {
		t.Fatal("expected refresh token to be removed")
	}
	if _, err := token.ValidateToken(ctx, access); err == nil {
		t.Fatal("access token should be rejected after logout")
	}
	if _, err := token.ValidateRefreshToken(ctx, refresh); err == nil {
		t.Fatal("refresh token should be rejected after logout")
	}
}

func newTestAuthToken(t *testing.T) *AuthToken {
	t.Helper()
	provider := authnJwt.NewProvider()
	authenticator, err := provider.NewAuthenticator(
		context.Background(),
		authn.WithSigningKey([]byte("test-secret")),
		authn.WithSigningMethod("HS256"),
		authn.WithIssuer("go-auth"),
		authn.WithAudience("go-auth-api"),
		authn.WithTokenExpiration(time.Hour),
		authn.WithRefreshTokenExpiration(24*time.Hour),
		authn.WithUserFactory(func(claims *authn.AuthClaims) authn.SecurityUser {
			return nil
		}),
	)
	if err != nil {
		t.Fatalf("new authenticator: %v", err)
	}
	return newAuthTokenWithStore(newMemoryTokenStore(), log.NewStdLogger(io.Discard), authenticator, "uat_", "urt_")
}

type memoryTokenStore struct {
	mu     sync.Mutex
	values map[string]memoryTokenValue
	now    func() time.Time
}

type memoryTokenValue struct {
	value     string
	expiresAt time.Time
}

func newMemoryTokenStore() *memoryTokenStore {
	return &memoryTokenStore{
		values: make(map[string]memoryTokenValue),
		now:    time.Now,
	}
}

func (s *memoryTokenStore) Set(ctx context.Context, key string, value string, expiration time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	expiresAt := time.Time{}
	if expiration > 0 {
		expiresAt = s.now().Add(expiration)
	}
	s.values[key] = memoryTokenValue{value: value, expiresAt: expiresAt}
	return nil
}

func (s *memoryTokenStore) Get(ctx context.Context, key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.values[key]
	if !ok {
		return "", redis.Nil
	}
	if !value.expiresAt.IsZero() && s.now().After(value.expiresAt) {
		delete(s.values, key)
		return "", redis.Nil
	}
	return value.value, nil
}

func (s *memoryTokenStore) Del(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.values, key)
	return nil
}

func (s *memoryTokenStore) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.Get(ctx, key)
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	return err == nil, err
}
