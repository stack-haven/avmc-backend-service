package data

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"backend-service/app/platform/admin/internal/data/ent/gen"
	"backend-service/app/platform/admin/internal/data/ent/gen/privacy"
	entviewer "backend-service/app/platform/admin/internal/data/ent/viewer"
	"backend-service/pkg/auth/authn"
)

type testSecurityUser struct {
	subject string
	tenant  string
}

func (u testSecurityUser) Name() string                           { return "test" }
func (u testSecurityUser) ParseFromContext(context.Context) error { return nil }
func (u testSecurityUser) GetSubject() string                     { return u.subject }
func (u testSecurityUser) GetObject() string                      { return "" }
func (u testSecurityUser) GetAction() string                      { return "" }
func (u testSecurityUser) GetTenant() string                      { return u.tenant }

func tenantContext(tenantID uint32) context.Context {
	return tenantUserContext(tenantID, 1)
}

func tenantUserContext(tenantID, userID uint32) context.Context {
	return authn.ContextWithAuthUser(context.Background(), testSecurityUser{
		subject: strconv.FormatUint(uint64(userID), 10),
		tenant:  strconv.FormatUint(uint64(tenantID), 10),
	})
}

func systemContext() context.Context {
	return entviewer.NewSystemContext(context.Background())
}

func TestRequireTenantID(t *testing.T) {
	t.Parallel()

	if _, err := requireTenantID(context.Background()); err == nil {
		t.Fatal("requireTenantID() error = nil without authenticated tenant")
	}
	if got, err := requireTenantID(tenantContext(7)); err != nil || got != 7 {
		t.Fatalf("requireTenantID() = %d, %v; want 7, nil", got, err)
	}
}

func TestTenantPrivacyPolicyFiltersAndInjectsTenant(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()

	ctx1 := tenantContext(1)
	ctx2 := tenantContext(2)
	systemCtx := systemContext()

	user1 := client.User.Create().
		SetName("tenant-1-user").
		SetPassword("password").
		SaveX(ctx1)
	if user1.TenantID != 1 {
		t.Fatalf("tenant 1 user tenant_id = %d, want 1", user1.TenantID)
	}

	user2 := client.User.Create().
		SetName("tenant-2-user").
		SetPassword("password").
		SaveX(ctx2)
	if user2.TenantID != 2 {
		t.Fatalf("tenant 2 user tenant_id = %d, want 2", user2.TenantID)
	}

	if got := client.User.Query().CountX(ctx1); got != 1 {
		t.Fatalf("tenant 1 user count = %d, want 1", got)
	}
	if got := client.User.Query().CountX(ctx2); got != 1 {
		t.Fatalf("tenant 2 user count = %d, want 1", got)
	}
	if got := client.User.Query().CountX(systemCtx); got != 2 {
		t.Fatalf("system user count = %d, want 2", got)
	}

	if _, err := client.User.Query().Count(context.Background()); !errors.Is(err, privacy.Deny) {
		t.Fatalf("query without tenant error = %v, want privacy deny", err)
	}
	if _, err := client.User.Create().
		SetTenantID(2).
		SetName("tenant-mismatch-user").
		SetPassword("password").
		Save(ctx1); !errors.Is(err, privacy.Deny) {
		t.Fatalf("create with mismatched tenant error = %v, want privacy deny", err)
	}

	if _, err := client.User.UpdateOneID(user1.ID).SetNickname("blocked").Save(ctx2); !gen.IsNotFound(err) {
		t.Fatalf("cross tenant update error = %v, want not found", err)
	}
	if _, err := client.User.UpdateOneID(user1.ID).SetNickname("allowed").Save(ctx1); err != nil {
		t.Fatalf("same tenant update: %v", err)
	}
}
