package static_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"backend-service/app/evie/tool/pkg/credential"
	"backend-service/app/evie/tool/pkg/credential/static"
)

func TestStaticProvider_HappyPath(t *testing.T) {
	p, err := static.New(static.Config{
		DefaultTenant: "demo",
		Users: []static.User{
			{Token: "alice-token", TenantID: "t1", UserID: "u-alice", UserName: "Alice"},
			{Token: "admin-token", UserID: "u-admin", UserType: 1},
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	t.Run("explicit tenant", func(t *testing.T) {
		id, err := p.Authenticate(context.Background(), "alice-token")
		if err != nil {
			t.Fatalf("Authenticate: %v", err)
		}
		if id.TenantID != "t1" || id.UserID != "u-alice" || id.UserName != "Alice" {
			t.Errorf("identity mismatch: %+v", id)
		}
	})

	t.Run("default tenant", func(t *testing.T) {
		id, _ := p.Authenticate(context.Background(), "admin-token")
		if id.TenantID != "demo" || id.UserID != "u-admin" {
			t.Errorf("default tenant not applied: %+v", id)
		}
	})

	t.Run("case-insensitive", func(t *testing.T) {
		_, err := p.Authenticate(context.Background(), "ALICE-TOKEN")
		if err != nil {
			t.Errorf("expected case-insensitive match, got %v", err)
		}
	})
}

func TestStaticProvider_NotFound(t *testing.T) {
	p, _ := static.New(static.Config{
		Users: []static.User{{Token: "known"}},
	})
	_, err := p.Authenticate(context.Background(), "unknown")
	if !errors.Is(err, credential.ErrTokenNotFound) {
		t.Errorf("err = %v, want ErrTokenNotFound", err)
	}
	_, err = p.Authenticate(context.Background(), "")
	if !errors.Is(err, credential.ErrTokenNotFound) {
		t.Errorf("empty token should return ErrTokenNotFound, got %v", err)
	}
}

func TestStaticProvider_EmptyConfigErrors(t *testing.T) {
	_, err := static.New(static.Config{})
	if !errors.Is(err, credential.ErrInvalidConfig) {
		t.Errorf("empty config should ErrInvalidConfig, got %v", err)
	}

	_, err = static.New(static.Config{Users: []static.User{{Token: ""}}})
	if !errors.Is(err, credential.ErrInvalidConfig) {
		t.Errorf("all-empty tokens should ErrInvalidConfig, got %v", err)
	}
}

func TestStaticProvider_AddUser(t *testing.T) {
	p, _ := static.New(static.Config{
		Users: []static.User{{Token: "init"}},
	})
	p.AddUser(static.User{Token: "added", TenantID: "t1", UserID: "u-new"})
	id, err := p.Authenticate(context.Background(), "added")
	if err != nil {
		t.Fatalf("Authenticate after Add: %v", err)
	}
	if id.UserID != "u-new" {
		t.Errorf("Added user mismatch: %+v", id)
	}
}

func TestStaticProvider_Name(t *testing.T) {
	p, _ := static.New(static.Config{Users: []static.User{{Token: "x"}}})
	if p.Name() != "static" {
		t.Errorf("Name() = %q, want static", p.Name())
	}
}

func TestStaticProvider_ExpiryPreserved(t *testing.T) {
	exp := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	p, _ := static.New(static.Config{
		Users: []static.User{{Token: "x", TenantID: "t", ExpiresAt: exp}},
	})
	id, _ := p.Authenticate(context.Background(), "x")
	if !id.ExpiresAt.Equal(exp) {
		t.Errorf("ExpiresAt = %v, want %v", id.ExpiresAt, exp)
	}
}
