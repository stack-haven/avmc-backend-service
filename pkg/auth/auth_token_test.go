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

func TestAuthTokenSupportsMultipleConcurrentSessions(t *testing.T) {
	ctx := context.Background()
	token := newTestAuthToken(t)

	access, refresh, err := token.GenerateToken(ctx, AuthTokenInfo{
		UserId:   42,
		Username: "tester",
		TenantID: 1,
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

	secondAccess, secondRefresh, err := token.GenerateToken(ctx, AuthTokenInfo{
		UserId:   42,
		Username: "tester",
		TenantID: 1,
	})
	if err != nil {
		t.Fatalf("generate second session: %v", err)
	}
	if secondAccess == access {
		t.Fatal("second access token should differ")
	}
	if _, err := token.ValidateToken(ctx, access); err != nil {
		t.Fatalf("first access token should remain valid: %v", err)
	}
	if _, err := token.ValidateToken(ctx, secondAccess); err != nil {
		t.Fatalf("second access token should validate: %v", err)
	}
	if _, err := token.ValidateRefreshToken(ctx, secondRefresh); err != nil {
		t.Fatalf("second refresh token should validate: %v", err)
	}
	sessions, err := token.ListUserSessions(ctx, 1, 42)
	if err != nil || len(sessions) != 2 {
		t.Fatalf("sessions = %d, %v; want 2", len(sessions), err)
	}
	if err := token.RevokeSession(ctx, 2, sessions[0].ID); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("cross-tenant revoke error = %v, want ErrSessionNotFound", err)
	}
	if _, err := token.ValidateToken(ctx, access); err != nil {
		t.Fatalf("cross-tenant revoke affected first session: %v", err)
	}
}

func TestAuthTokenRotatesAndRevokesOneSessionIndependently(t *testing.T) {
	ctx := context.Background()
	token := newTestAuthToken(t)
	authInfo := AuthTokenInfo{UserId: 42, Username: "tester", TenantID: 1}
	firstAccess, firstRefresh, err := token.GenerateToken(ctx, authInfo)
	if err != nil {
		t.Fatalf("generate first session: %v", err)
	}
	secondAccess, _, err := token.GenerateToken(ctx, authInfo)
	if err != nil {
		t.Fatalf("generate second session: %v", err)
	}
	firstClaims, err := token.ValidateRefreshToken(ctx, firstRefresh)
	if err != nil {
		t.Fatalf("validate first refresh: %v", err)
	}
	rotatedAccess, rotatedRefresh, err := token.RotateSessionToken(ctx, authInfo, firstClaims.GetID())
	if err != nil {
		t.Fatalf("rotate first session: %v", err)
	}
	if _, err := token.ValidateToken(ctx, firstAccess); err == nil {
		t.Fatal("old access token should be invalid after rotation")
	}
	if _, err := token.ValidateRefreshToken(ctx, firstRefresh); err == nil {
		t.Fatal("old refresh token should be invalid after rotation")
	}
	if _, err := token.ValidateToken(ctx, rotatedAccess); err != nil {
		t.Fatalf("rotated access token: %v", err)
	}
	if _, err := token.ValidateRefreshToken(ctx, rotatedRefresh); err != nil {
		t.Fatalf("rotated refresh token: %v", err)
	}
	if _, err := token.ValidateToken(ctx, secondAccess); err != nil {
		t.Fatalf("second session should remain valid: %v", err)
	}
	if err := token.RevokeSession(ctx, 1, firstClaims.GetID()); err != nil {
		t.Fatalf("revoke first session: %v", err)
	}
	if _, err := token.ValidateToken(ctx, rotatedAccess); err == nil {
		t.Fatal("revoked session should be invalid")
	}
	if _, err := token.ValidateToken(ctx, secondAccess); err != nil {
		t.Fatalf("second session should remain valid after revoke: %v", err)
	}
}

func TestAuthTokenRemoveRevokesAccessAndRefreshTokens(t *testing.T) {
	ctx := context.Background()
	token := newTestAuthToken(t)

	access, refresh, err := token.GenerateToken(ctx, AuthTokenInfo{
		UserId:   7,
		Username: "tester",
		TenantID: 2,
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

func TestAuthTokenCapsSessionAtTenantExpiration(t *testing.T) {
	ctx := context.Background()
	token := newTestAuthToken(t)
	tenantExpiresAt := time.Now().Add(2 * time.Minute)

	access, _, err := token.GenerateToken(ctx, AuthTokenInfo{
		UserId:          9,
		Username:        "expiring-user",
		TenantID:        3,
		TenantExpiresAt: &tenantExpiresAt,
	})
	if err != nil {
		t.Fatalf("generate expiring tenant token: %v", err)
	}
	claims, err := token.ValidateToken(ctx, access)
	if err != nil {
		t.Fatalf("validate expiring tenant token: %v", err)
	}
	session, err := token.GetSession(ctx, claims.GetID())
	if err != nil {
		t.Fatalf("get expiring tenant session: %v", err)
	}
	if session.ExpiresAt.After(tenantExpiresAt.Add(time.Second)) {
		t.Fatalf("session expires at %v, after tenant expiration %v", session.ExpiresAt, tenantExpiresAt)
	}
	if session.ExpiresAt.Before(tenantExpiresAt.Add(-2 * time.Second)) {
		t.Fatalf("session expiration was shortened unexpectedly: %v", session.ExpiresAt)
	}
}

func TestAuthTokenCarriesPlatformOperatorClaimAcrossRotation(t *testing.T) {
	ctx := context.Background()
	token := newTestAuthToken(t)
	info := AuthTokenInfo{
		UserId:           10,
		Username:         "platform-user",
		TenantID:         1,
		PlatformOperator: true,
	}
	access, refresh, err := token.GenerateToken(ctx, info)
	if err != nil {
		t.Fatalf("generate platform token: %v", err)
	}
	claims, err := token.ValidateToken(ctx, access)
	if err != nil || !claims.IsPlatformOperator() {
		t.Fatalf("platform access claims = %v, %v", claims, err)
	}
	refreshClaims, err := token.ValidateRefreshToken(ctx, refresh)
	if err != nil || !refreshClaims.IsPlatformOperator() {
		t.Fatalf("platform refresh claims = %v, %v", refreshClaims, err)
	}
	rotated, _, err := token.RotateSessionToken(ctx, info, refreshClaims.GetID())
	if err != nil {
		t.Fatalf("rotate platform token: %v", err)
	}
	rotatedClaims, err := token.ValidateToken(ctx, rotated)
	if err != nil || !rotatedClaims.IsPlatformOperator() {
		t.Fatalf("rotated platform claims = %v, %v", rotatedClaims, err)
	}
}

func TestEffectiveTokenExpiration(t *testing.T) {
	future := time.Now().Add(5 * time.Minute)
	if got := effectiveTokenExpiration(time.Hour, &future); got > 5*time.Minute || got < 4*time.Minute+58*time.Second {
		t.Fatalf("effective expiration = %v, want about 5m", got)
	}
	if got := effectiveTokenExpiration(time.Minute, &future); got != time.Minute {
		t.Fatalf("effective expiration = %v, want default 1m", got)
	}
	past := time.Now().Add(-time.Minute)
	if got := effectiveTokenExpiration(time.Hour, &past); got != time.Second {
		t.Fatalf("expired tenant duration = %v, want 1s", got)
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
	sets   map[string]map[string]struct{}
	now    func() time.Time
}

type memoryTokenValue struct {
	value     string
	expiresAt time.Time
}

func newMemoryTokenStore() *memoryTokenStore {
	return &memoryTokenStore{
		values: make(map[string]memoryTokenValue),
		sets:   make(map[string]map[string]struct{}),
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

func (s *memoryTokenStore) SetAdd(_ context.Context, key string, values ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sets[key] == nil {
		s.sets[key] = make(map[string]struct{})
	}
	for _, value := range values {
		s.sets[key][value] = struct{}{}
	}
	return nil
}

func (s *memoryTokenStore) SetRemove(_ context.Context, key string, values ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, value := range values {
		delete(s.sets[key], value)
	}
	return nil
}

func (s *memoryTokenStore) SetMembers(_ context.Context, key string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]string, 0, len(s.sets[key]))
	for value := range s.sets[key] {
		result = append(result, value)
	}
	return result, nil
}

func (s *memoryTokenStore) Expire(_ context.Context, _ string, _ time.Duration) error {
	return nil
}
