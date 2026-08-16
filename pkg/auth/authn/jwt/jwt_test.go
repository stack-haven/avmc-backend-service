package jwt

import (
	"context"
	"testing"
	"time"

	"backend-service/pkg/auth/authn"
)

func newTestAuthenticator(t *testing.T, opts ...authn.Option) authn.Authenticator {
	t.Helper()
	base := []authn.Option{
		authn.WithSigningKey([]byte("test-secret-key")),
		authn.WithSigningMethod("HS256"),
		authn.WithIssuer("test-issuer"),
		authn.WithAudience("test-audience"),
		authn.WithTokenExpiration(time.Hour),
		authn.WithRefreshTokenExpiration(24 * time.Hour),
		authn.WithUserFactory(func(*authn.AuthClaims) authn.SecurityUser { return nil }),
	}
	base = append(base, opts...)
	a, err := authn.NewAuthenticator("jwt", context.Background(), base...)
	if err != nil {
		t.Fatalf("new authenticator: %v", err)
	}
	return a
}

func TestCreateAndValidateToken(t *testing.T) {
	a := newTestAuthenticator(t)

	claims := authn.AuthClaims{}.
		SetID("session-1").
		SetSubject("42").
		SetTenant("1")

	token, err := a.CreateToken(context.Background(), claims, time.Hour)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	if token == "" {
		t.Fatal("empty token")
	}

	got, err := a.ValidateToken(context.Background(), token)
	if err != nil {
		t.Fatalf("validate token: %v", err)
	}
	if got.GetSubject() != "42" {
		t.Fatalf("subject = %s, want 42", got.GetSubject())
	}
	if got.GetTenant() != "1" {
		t.Fatalf("tenant = %s, want 1", got.GetTenant())
	}
	if got.GetID() != "session-1" {
		t.Fatalf("id = %s, want session-1", got.GetID())
	}
}

func TestValidateTokenRejectsWrongKey(t *testing.T) {
	a := newTestAuthenticator(t)
	claims := authn.AuthClaims{}.SetSubject("42")
	token, _ := a.CreateToken(context.Background(), claims, time.Hour)

	// 用不同密钥的认证器验证
	other := newTestAuthenticator(t, authn.WithSigningKey([]byte("different-key")))
	if _, err := other.ValidateToken(context.Background(), token); err == nil {
		t.Fatal("expected signature error with wrong key")
	}
}

func TestValidateTokenRejectsWrongIssuer(t *testing.T) {
	a := newTestAuthenticator(t)
	claims := authn.AuthClaims{}.SetSubject("42")
	token, _ := a.CreateToken(context.Background(), claims, time.Hour)

	// 用不同 issuer 的认证器验证
	other := newTestAuthenticator(t, authn.WithIssuer("other-issuer"))
	if _, err := other.ValidateToken(context.Background(), token); err == nil {
		t.Fatal("expected issuer error")
	}
}

func TestValidateTokenLeeway(t *testing.T) {
	// 签发一个即将过期的 token（过期时间很短）
	a := newTestAuthenticator(t, authn.WithTokenExpiration(time.Second))
	claims := authn.AuthClaims{}.SetSubject("42")
	token, _ := a.CreateToken(context.Background(), claims, time.Second)

	// 等待超过过期时间
	time.Sleep(1100 * time.Millisecond)

	// 无 leeway：应拒绝（过期）
	noLeeway := newTestAuthenticator(t)
	if _, err := noLeeway.ValidateToken(context.Background(), token); err == nil {
		t.Fatal("expected expired error without leeway")
	}

	// 有 leeway：应放行（时钟偏移容差）
	withLeeway := newTestAuthenticator(t, authn.WithLeeway(5*time.Second))
	if _, err := withLeeway.ValidateToken(context.Background(), token); err != nil {
		t.Fatalf("expected accept with leeway: %v", err)
	}
}

func TestKeyRotationWithKid(t *testing.T) {
	// 旧密钥（kid=old）签发 token
	oldAuth := newTestAuthenticator(t, authn.WithKeyID("old"))
	claims := authn.AuthClaims{}.SetSubject("42")
	token, _ := oldAuth.CreateToken(context.Background(), claims, time.Hour)

	// 新密钥（kid=new）的认证器，配置 VerificationKeys 含旧密钥
	newAuth := newTestAuthenticator(t,
		authn.WithSigningKey([]byte("new-secret-key")),
		authn.WithKeyID("new"),
		authn.WithVerificationKeys(map[string]interface{}{
			"new": []byte("new-secret-key"),
			"old": []byte("test-secret-key"),
		}),
	)
	if _, err := newAuth.ValidateToken(context.Background(), token); err != nil {
		t.Fatalf("key rotation should accept old token: %v", err)
	}
}

func TestRefreshTokenRequiresRefreshType(t *testing.T) {
	a := newTestAuthenticator(t)

	// access 类型 token 不能刷新
	access := authn.AuthClaims{}.
		SetSubject("42").
		SetTokenType(authn.TokenTypeAccess)
	accessToken, _ := a.CreateToken(context.Background(), access, time.Hour)
	if _, err := a.RefreshToken(context.Background(), accessToken); err == nil {
		t.Fatal("expected error: access token cannot be refreshed")
	}

	// refresh 类型 token 可以刷新
	refresh := authn.AuthClaims{}.
		SetSubject("42").
		SetTokenType(authn.TokenTypeRefresh)
	refreshToken, _ := a.CreateToken(context.Background(), refresh, time.Hour)
	if _, err := a.RefreshToken(context.Background(), refreshToken); err != nil {
		t.Fatalf("refresh token should refresh: %v", err)
	}
}

func TestProviderRegistration(t *testing.T) {
	// jwt Provider 已通过 init 注册
	if _, ok := authn.GetProvider("jwt"); !ok {
		t.Fatal("jwt provider should be registered")
	}

	// 按名称创建
	a, err := authn.NewAuthenticator("jwt", context.Background(),
		authn.WithSigningKey([]byte("k")),
		authn.WithSigningMethod("HS256"),
	)
	if err != nil {
		t.Fatalf("new by name: %v", err)
	}
	if a.Name() != "jwt" {
		t.Fatalf("name = %s, want jwt", a.Name())
	}

	// 未知 provider 报错
	if _, err := authn.NewAuthenticator("unknown", context.Background()); err == nil {
		t.Fatal("expected error for unknown provider")
	}
}
