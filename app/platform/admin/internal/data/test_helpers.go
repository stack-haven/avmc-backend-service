package data

import (
	"context"
	"database/sql"
	"strconv"
	"testing"

	"backend-service/app/platform/admin/internal/data/ent/gen"
	"backend-service/app/platform/admin/internal/data/ent/gen/migrate"
	"backend-service/app/platform/admin/internal/data/ent/gen/tenant"
	entviewer "backend-service/app/platform/admin/internal/data/ent/viewer"
	"backend-service/pkg/auth/authn"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/schema"
	_ "github.com/glebarez/sqlite"
)

// testSecurityUser is a minimal SecurityUser used to carry tenant/user
// identity through authn.ContextWithAuthUser in repo tests.
type testSecurityUser struct {
	userID   uint32
	tenantID uint32
}

func (u testSecurityUser) Name() string                           { return "test" }
func (u testSecurityUser) ParseFromContext(context.Context) error { return nil }
func (u testSecurityUser) GetSubject() string                     { return strconv.FormatUint(uint64(u.userID), 10) }
func (u testSecurityUser) GetObject() string                      { return "" }
func (u testSecurityUser) GetAction() string                      { return "" }
func (u testSecurityUser) GetTenant() string {
	return strconv.FormatUint(uint64(u.tenantID), 10)
}

// tenantContext returns a context with the given tenant ID set for testing.
// It injects both the Ent viewer tenant (privacy filters) and the auth user
// tenant (used by BaseRepo.RequireTenantID).
func tenantContext(tenantID uint32) context.Context {
	return tenantUserContext(tenantID, 1)
}

// tenantUserContext returns a context with the given tenant ID and user ID
// set for testing.
func tenantUserContext(tenantID, userID uint32) context.Context {
	ctx := entviewer.NewTenantContext(context.Background(), tenantID)
	return authn.ContextWithAuthUser(ctx, testSecurityUser{userID: userID, tenantID: tenantID})
}

// systemContext returns a context that bypasses tenant privacy filters for testing.
func systemContext() context.Context {
	return entviewer.NewSystemContext(context.Background())
}

// newTestClient creates a temporary SQLite Ent client for testing.
// Ent requires foreign_keys ON during auto-migration, but the repo tests
// create root nodes that rely on the default parent_id=0 (the repos perform
// their own cross-tenant/cycle validation), so FK enforcement is disabled
// after migration.
func newTestClient(t *testing.T) *gen.Client {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(1)
	drv := entsql.OpenDB(dialect.SQLite, db)
	client := gen.NewClient(gen.Driver(drv))
	// Run auto-migration.
	tables, err := schema.CopyTables(migrate.Tables)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrate.Create(context.Background(), client.Schema, tables); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=OFF"); err != nil {
		t.Fatal(err)
	}
	return client
}

// ptr returns a pointer to v for use in test protobuf field assignments.
func ptr[T any](v T) *T {
	return &v
}

// seedTenantMenuPermissionGroup creates a tenant (if missing) and a menu
// permission group bound to it, covering the given menu IDs. The group is
// given a current version so that GetTenantEffectiveMenuIDs resolves the
// menus (the effective menu set comes from the group's current version).
func seedTenantMenuPermissionGroup(t *testing.T, client *gen.Client, tenantID uint32, menuIDs ...uint32) {
	t.Helper()
	ctx := systemContext()
	if !client.Tenant.Query().Where(tenant.IDEQ(tenantID)).ExistX(ctx) {
		client.Tenant.Create().
			SetID(tenantID).
			SetName("test-tenant").
			SetCode("test-tenant").
			SaveX(ctx)
	}
	group := client.TenantMenuPermissionGroup.Create().
		SetName("test-group").
		SetCode("test-code").
		SetStatus(1).
		AddTenantIDs(tenantID).
		SaveX(ctx)
	version := client.TenantMenuPermissionGroupVersion.Create().
		SetGroupID(group.ID).
		SetVersion(1).
		SetState(1).
		AddMenuIDs(menuIDs...).
		SaveX(ctx)
	client.TenantMenuPermissionGroup.UpdateOneID(group.ID).
		SetCurrentVersionID(version.ID).
		SaveX(ctx)
}

// assertRoleMenuIDs verifies that a role has the expected menu IDs assigned.
func assertRoleMenuIDs(t *testing.T, client *gen.Client, roleID uint32, expected []uint32) {
	t.Helper()
	ctx := systemContext()
	roleEnt := client.Role.GetX(ctx, roleID)
	menus, err := roleEnt.QueryMenus().All(ctx)
	if err != nil {
		t.Fatalf("query menus for role %d: %v", roleID, err)
	}
	got := make([]uint32, len(menus))
	for i, m := range menus {
		got[i] = m.ID
	}
	if len(expected) != len(got) {
		t.Fatalf("role %d menu count = %d, want %d; got=%v expected=%v", roleID, len(got), len(expected), got, expected)
	}
	for i := range got {
		if got[i] != expected[i] {
			t.Fatalf("role %d menu[%d] = %d, want %d; got=%v expected=%v", roleID, i, got[i], expected[i], got, expected)
		}
	}
}
