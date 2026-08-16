package authn

import (
	"testing"
	"time"
)

func TestClaimsSettersAndGetters(t *testing.T) {
	now := time.Now()
	claims := AuthClaims{}.
		SetID("session-1").
		SetSubject("42").
		SetTenant("1").
		SetIssuer("issuer").
		SetAudience([]string{"aud"}).
		SetPlatformOperator(true).
		SetTokenType(TokenTypeAccess).
		SetScope("read").
		SetNonce("nonce-1").
		SetExpiresAt(now)

	if claims.GetID() != "session-1" {
		t.Fatalf("id = %s", claims.GetID())
	}
	if claims.GetSubject() != "42" {
		t.Fatalf("subject = %s", claims.GetSubject())
	}
	if claims.GetTenant() != "1" {
		t.Fatalf("tenant = %s", claims.GetTenant())
	}
	if claims.GetIssuer() != "issuer" {
		t.Fatalf("issuer = %s", claims.GetIssuer())
	}
	if !claims.IsPlatformOperator() {
		t.Fatal("expected platform operator true")
	}
	if claims.GetTokenType() != TokenTypeAccess {
		t.Fatalf("token type = %s", claims.GetTokenType())
	}
}

func TestClaimsNilSafety(t *testing.T) {
	var claims *AuthClaims
	if claims.GetSubject() != "" {
		t.Fatal("nil claims subject should be empty")
	}
	if claims.IsPlatformOperator() {
		t.Fatal("nil claims should not be platform operator")
	}
	if claims.GetTokenType() != "" {
		t.Fatal("nil claims token type should be empty")
	}
	if claims.IsExpired() {
		t.Fatal("nil claims should not be expired")
	}
}

func TestClaimsTokenTypeRoundTrip(t *testing.T) {
	claims := AuthClaims{}.SetTokenType(TokenTypeRefresh)
	if claims.GetTokenType() != TokenTypeRefresh {
		t.Fatalf("token type = %s, want refresh", claims.GetTokenType())
	}
}
