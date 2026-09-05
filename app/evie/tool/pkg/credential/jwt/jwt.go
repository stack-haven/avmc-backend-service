// Package jwt provides a credential.Provider that validates
// HMAC-SHA256 or RSA-signed JSON Web Tokens.
//
// # Use case
//
// Self-hosted / standalone deployments where no external identity
// store is available can mint short-lived JWTs and let this provider
// verify them locally. The implementation is self-contained (no
// external JWT library dependency) and covers HS256 / RS256 with
// issuer/audience checks and configurable JSON-claim mapping.
//
// # Example
//
//	p, _ := jwt.New(jwt.Config{
//	    Algorithm: "HS256",
//	    Secret:    []byte("dev-secret"),
//	    Issuer:    "evie-tool",
//	    Fields: jwt.FieldMapper{
//	        TenantID: "tenant_id",
//	        UserID:   "sub",
//	        UserName: "name",
//	    },
//	})
package jwt

import (
	"context"
	"crypto"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"

	"backend-service/app/evie/tool/pkg/credential"
)

// FieldMapper is an alias for credential.FieldMapper.
type FieldMapper = credential.FieldMapper

// Config configures a JWT-based Provider.
type Config struct {
	// Algorithm is "HS256" or "RS256". Defaults to HS256.
	Algorithm string

	// Secret is the HMAC secret for HS256. Mutually exclusive with
	// PublicKeyPEM (RS256).
	Secret []byte

	// PublicKeyPEM is a PEM-encoded RSA public key for RS256.
	PublicKeyPEM []byte

	// Issuer, when non-empty, requires the token's "iss" claim to match.
	Issuer string

	// Audience, when non-empty, requires the token's "aud" claim to match.
	Audience string

	// Leeway is the clock skew tolerance applied to exp/nbf (default 0).
	Leeway time.Duration

	// Fields describes how to project JWT claims into CallerIdentity.
	// Top-level JSON keys (e.g. "tenant_id", "sub", "name"); nested
	// paths use dot notation.
	Fields FieldMapper
}

// Provider validates JWTs locally and returns the CallerIdentity.
type Provider struct {
	cfg Config
	pub *rsa.PublicKey
}

// New constructs a JWT Provider.
func New(cfg Config) (*Provider, error) {
	algo := cfg.Algorithm
	if algo == "" {
		algo = "HS256"
	}
	p := &Provider{cfg: cfg}
	switch algo {
	case "HS256":
		if len(cfg.Secret) == 0 {
			return nil, fmt.Errorf("%w: HS256 requires Secret", credential.ErrInvalidConfig)
		}
	case "RS256":
		if len(cfg.PublicKeyPEM) == 0 {
			return nil, fmt.Errorf("%w: RS256 requires PublicKeyPEM", credential.ErrInvalidConfig)
		}
		pub, err := parseRSAPublicKey(cfg.PublicKeyPEM)
		if err != nil {
			return nil, fmt.Errorf("%w: parse RSA key: %v", credential.ErrInvalidConfig, err)
		}
		p.pub = pub
	default:
		return nil, fmt.Errorf("%w: unsupported algorithm %q", credential.ErrInvalidConfig, algo)
	}
	return p, nil
}

// Name implements credential.Provider.
func (p *Provider) Name() string { return "jwt" }

// Authenticate verifies the token and returns the CallerIdentity.
//
// Implementation: self-contained JWT parsing + signature verification
// (no third-party library). Supports HS256 and RS256.
func (p *Provider) Authenticate(_ context.Context, token string) (*credential.CallerIdentity, error) {
	if token == "" {
		return nil, credential.ErrTokenNotFound
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("%w: malformed JWT (expected 3 parts)", credential.ErrTokenInvalid)
	}
	header, payload, sig := parts[0], parts[1], parts[2]

	if err := p.verifySignature(header, payload, sig); err != nil {
		return nil, err
	}

	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("%w: decode payload: %v", credential.ErrTokenInvalid, err)
	}
	var claims map[string]any
	if err := json.Unmarshal(raw, &claims); err != nil {
		return nil, fmt.Errorf("%w: parse claims: %v", credential.ErrTokenInvalid, err)
	}

	if exp, ok := credential.ParseExpiresAt(claims["exp"]); ok && time.Now().After(exp.Add(p.cfg.Leeway)) {
		return nil, fmt.Errorf("%w: token expired", credential.ErrTokenInvalid)
	}
	if nbf, ok := credential.ParseExpiresAt(claims["nbf"]); ok && time.Now().Before(nbf.Add(-p.cfg.Leeway)) {
		return nil, fmt.Errorf("%w: token not yet valid", credential.ErrTokenInvalid)
	}
	if p.cfg.Issuer != "" {
		if iss, _ := claims["iss"].(string); iss != p.cfg.Issuer {
			return nil, fmt.Errorf("%w: bad issuer", credential.ErrTokenInvalid)
		}
	}
	if p.cfg.Audience != "" {
		if !audienceMatches(claims["aud"], p.cfg.Audience) {
			return nil, fmt.Errorf("%w: bad audience", credential.ErrTokenInvalid)
		}
	}

	id := credential.MapFromMapper(claims, p.cfg.Fields)
	id.AccessToken = token
	return &id, nil
}

func (p *Provider) verifySignature(header, payload, sig string) error {
	hBytes, err := base64.RawURLEncoding.DecodeString(header)
	if err != nil {
		return fmt.Errorf("%w: decode header: %v", credential.ErrTokenInvalid, err)
	}
	var h struct {
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(hBytes, &h); err != nil {
		return fmt.Errorf("%w: parse header: %v", credential.ErrTokenInvalid, err)
	}
	algo := p.cfg.Algorithm
	if algo == "" {
		algo = "HS256"
	}
	if h.Alg != algo {
		return fmt.Errorf("%w: alg mismatch (got %q want %q)", credential.ErrTokenInvalid, h.Alg, algo)
	}
	sigBytes, err := base64.RawURLEncoding.DecodeString(sig)
	if err != nil {
		return fmt.Errorf("%w: decode signature: %v", credential.ErrTokenInvalid, err)
	}
	signing := header + "." + payload

	switch algo {
	case "HS256":
		mac := hmac.New(sha256.New, p.cfg.Secret)
		mac.Write([]byte(signing))
		if !hmac.Equal(mac.Sum(nil), sigBytes) {
			return fmt.Errorf("%w: bad signature", credential.ErrTokenInvalid)
		}
	case "RS256":
		h := sha256.Sum256([]byte(signing))
		if err := rsa.VerifyPKCS1v15(p.pub, crypto.SHA256, h[:], sigBytes); err != nil {
			return fmt.Errorf("%w: bad signature: %v", credential.ErrTokenInvalid, err)
		}
	}
	return nil
}

// audienceMatches returns true if claim aud (string or []any) contains
// the expected value.
func audienceMatches(aud any, expected string) bool {
	switch x := aud.(type) {
	case string:
		return x == expected
	case []any:
		for _, v := range x {
			if s, ok := v.(string); ok && s == expected {
				return true
			}
		}
	}
	return false
}

// parseRSAPublicKey parses a PEM-encoded RSA public key (PKIX or PKCS1).
func parseRSAPublicKey(pemBytes []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("no PEM block found")
	}
	if pub, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		rsaPub, ok := pub.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("PKIX key is not RSA")
		}
		return rsaPub, nil
	}
	if pub, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		return pub, nil
	}
	return nil, errors.New("failed to parse RSA public key")
}
