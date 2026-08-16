//go:build integration
// +build integration

package data

import (
	"context"
	"testing"

	"backend-service/app/platform/service/internal/data/ent/gen"
	"backend-service/app/platform/service/internal/data/ent/viewer"

	_ "github.com/mattn/go-sqlite3"
)

// TestEntPrivacyTenantIsolation demonstrates how to write integration tests
// that validate Ent Privacy rules using an in-memory SQLite database.
//
// This test verifies that FilterTenantRule correctly:
//  1. Rejects queries when no tenant context is present.
//  2. Allows queries scoped to a specific tenant.
//  3. Allows system context to bypass tenant filters.
func TestEntPrivacyTenantIsolation(t *testing.T) {
	// Use SQLite memory for fast, isolated tests.
	client, err := gen.Open("sqlite3", "file:ent_privacy_test?mode=memory&_fk=1")
	if err != nil {
		t.Fatalf("failed opening sqlite: %v", err)
	}
	defer client.Close()

	if err := client.Schema.Create(context.Background()); err != nil {
		t.Fatalf("failed creating schema: %v", err)
	}

	// Seed: create data for two tenants.
	ctx := viewer.NewSystemContext(context.Background())

	t1, err := client.Tenant.Create().
		SetName("Tenant A").SetCode("ta").SetLifecycleStatus(2).
		Save(ctx)
	if err != nil {
		t.Fatalf("create tenant A: %v", err)
	}
	t2, err := client.Tenant.Create().
		SetName("Tenant B").SetCode("tb").SetLifecycleStatus(2).
		Save(ctx)
	if err != nil {
		t.Fatalf("create tenant B: %v", err)
	}

	u1, err := client.User.Create().
		SetTenantID(t1.ID).SetName("user_a").SetPassword("hash").SetStatus(1).
		Save(ctx)
	if err != nil {
		t.Fatalf("create user A: %v", err)
	}
	u2, err := client.User.Create().
		SetTenantID(t2.ID).SetName("user_b").SetPassword("hash").SetStatus(1).
		Save(ctx)
	if err != nil {
		t.Fatalf("create user B: %v", err)
	}
	_ = u1
	_ = u2

	t.Run("system context sees all users", func(t *testing.T) {
		count, err := client.User.Query().Count(ctx)
		if err != nil {
			t.Fatalf("count users: %v", err)
		}
		if count != 2 {
			t.Errorf("system context: expected 2 users, got %d", count)
		}
	})

	t.Run("tenant-scoped context sees only own users", func(t *testing.T) {
		ctxA := viewer.NewTenantContext(context.Background(), t1.ID)
		users, err := client.User.Query().All(ctxA)
		if err != nil {
			t.Fatalf("query users for tenant A: %v", err)
		}
		if len(users) != 1 {
			t.Fatalf("tenant A: expected 1 user, got %d", len(users))
		}
		if users[0].Name != "user_a" {
			t.Errorf("tenant A: expected user_a, got %s", users[0].Name)
		}
	})

	t.Run("missing tenant context is denied", func(t *testing.T) {
		// No viewer set — Privacy rule should deny.
		_, err := client.User.Query().All(context.Background())
		if err == nil {
			t.Fatal("expected error when no tenant context is present")
		}
	})
}
