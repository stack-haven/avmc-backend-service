package jwt_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"backend-service/app/evie/tool/pkg/credential"
	credjwt "backend-service/app/evie/tool/pkg/credential/jwt"
)

// helper: mint a HS256 JWT with custom claims.
func mintHS256(t *testing.T, secret []byte, claims map[string]any) string {
	t.Helper()
	header := map[string]any{"alg": "HS256", "typ": "JWT"}
	hb, _ := json.Marshal(header)
	cb, _ := json.Marshal(claims)
	hp := base64.RawURLEncoding.EncodeToString(hb)
	cp := base64.RawURLEncoding.EncodeToString(cb)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(hp + "." + cp))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return hp + "." + cp + "." + sig
}

func TestJWT_HS256_HappyPath(t *testing.T) {
	secret := []byte("test-secret")
	exp := time.Now().Add(1 * time.Hour).Unix()
	tok := mintHS256(t, secret, map[string]any{
		"sub":      "u-1",
		"iss":      "test-issuer",
		"exp":      exp,
		"tenant_id": "t-1",
		"name":     "Alice",
	})

	p, err := credjwt.New(credjwt.Config{
		Algorithm: "HS256",
		Secret:    secret,
		Issuer:    "test-issuer",
		Audience:  "", // 测试里 token 不带 aud
		Fields: credential.FieldMapper{
			TenantID: "tenant_id", UserID: "sub", UserName: "name",
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	id, err := p.Authenticate(context.Background(), tok)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if id.TenantID != "t-1" || id.UserID != "u-1" || id.UserName != "Alice" {
		t.Errorf("identity mismatch: %+v", id)
	}
	if id.AccessToken != tok {
		t.Errorf("AccessToken should echo bearer")
	}
}

func TestJWT_HS256_BadSignature(t *testing.T) {
	tok := mintHS256(t, []byte("correct-secret"), map[string]any{"sub": "u-1"})
	p, _ := credjwt.New(credjwt.Config{
		Algorithm: "HS256",
		Secret:    []byte("wrong-secret"),
	})
	_, err := p.Authenticate(context.Background(), tok)
	if !errors.Is(err, credential.ErrTokenInvalid) {
		t.Errorf("err = %v, want ErrTokenInvalid", err)
	}
}

func TestJWT_HS256_Expired(t *testing.T) {
	secret := []byte("s")
	tok := mintHS256(t, secret, map[string]any{
		"sub": "u-1", "exp": time.Now().Add(-1 * time.Hour).Unix(),
	})
	p, _ := credjwt.New(credjwt.Config{Algorithm: "HS256", Secret: secret})
	_, err := p.Authenticate(context.Background(), tok)
	if !errors.Is(err, credential.ErrTokenInvalid) || !strings.Contains(err.Error(), "expired") {
		t.Errorf("err = %v, want expired ErrTokenInvalid", err)
	}
}

func TestJWT_HS256_BadIssuer(t *testing.T) {
	secret := []byte("s")
	tok := mintHS256(t, secret, map[string]any{"iss": "other", "exp": time.Now().Add(1 * time.Hour).Unix()})
	p, _ := credjwt.New(credjwt.Config{
		Algorithm: "HS256", Secret: secret, Issuer: "expected",
	})
	_, err := p.Authenticate(context.Background(), tok)
	if !errors.Is(err, credential.ErrTokenInvalid) || !strings.Contains(err.Error(), "issuer") {
		t.Errorf("err = %v, want issuer ErrTokenInvalid", err)
	}
}

func TestJWT_Malformed(t *testing.T) {
	p, _ := credjwt.New(credjwt.Config{Algorithm: "HS256", Secret: []byte("s")})
	tests := []string{"", "no-dots", "two.dots", "a.b.c", "YQ.YQ.YQ"} // last is "a"/"a"/"a" but alg mismatch
	for _, tok := range tests {
		_, err := p.Authenticate(context.Background(), tok)
		if !errors.Is(err, credential.ErrTokenNotFound) && !errors.Is(err, credential.ErrTokenInvalid) {
			t.Errorf("token %q: err = %v, want not-found or invalid", tok, err)
		}
	}
}

func TestJWT_AlgMismatch(t *testing.T) {
	// Mint a token with alg=none-ish (header says HS256 but we'll fake it)
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"u-1"}`))
	tok := header + "." + payload + ".AAAA"
	p, _ := credjwt.New(credjwt.Config{Algorithm: "HS256", Secret: []byte("s")})
	_, err := p.Authenticate(context.Background(), tok)
	if !errors.Is(err, credential.ErrTokenInvalid) || !strings.Contains(err.Error(), "alg mismatch") {
		t.Errorf("err = %v, want alg-mismatch ErrTokenInvalid", err)
	}
}

func TestJWT_NewValidation(t *testing.T) {
	tests := []struct {
		name string
		cfg  credjwt.Config
	}{
		{"unsupported algo", credjwt.Config{Algorithm: "ES256"}},
		{"HS256 missing secret", credjwt.Config{Algorithm: "HS256"}},
		{"RS256 missing key", credjwt.Config{Algorithm: "RS256"}},
		{"RS256 invalid PEM", credjwt.Config{Algorithm: "RS256", PublicKeyPEM: []byte("not pem")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := credjwt.New(tt.cfg)
			if !errors.Is(err, credential.ErrInvalidConfig) {
				t.Errorf("err = %v, want ErrInvalidConfig", err)
			}
		})
	}
}

func TestJWT_AudienceString(t *testing.T) {
	secret := []byte("s")
	tok := mintHS256(t, secret, map[string]any{
		"sub": "u", "aud": "evie-tool", "exp": time.Now().Add(1 * time.Hour).Unix(),
	})
	p, _ := credjwt.New(credjwt.Config{
		Algorithm: "HS256", Secret: secret, Audience: "evie-tool",
		Fields: credential.FieldMapper{UserID: "sub"},
	})
	id, err := p.Authenticate(context.Background(), tok)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if id.UserID != "u" {
		t.Errorf("identity mismatch: %+v", id)
	}
}
