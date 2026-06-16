package data

import (
	"context"
	stdsql "database/sql"
	"path/filepath"
	"testing"
)

func TestBackfillTenantTenant(t *testing.T) {
	db := newLegacyTenantTestDB(t)
	defer db.Close()

	for _, statement := range []string{
		"INSERT INTO users(tenant_id, name, email, phone) VALUES (11, 'admin', 'admin@example.com', '100')",
		"INSERT INTO roles(tenant_id, name) VALUES (12, 'admin')",
		"INSERT INTO posts(tenant_id, name) VALUES (13, 'developer')",
		"INSERT INTO depts(tenant_id, name) VALUES (14, 'engineering')",
		"INSERT INTO projects(tenant_id, name, code) VALUES (15, 'app', 'app')",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("seed legacy data: %v", err)
		}
	}

	if err := backfillTenantScope(context.Background(), db, 1); err != nil {
		t.Fatalf("backfillTenantScope: %v", err)
	}
	for table := range tenantUniqueColumns {
		var tenantID uint32
		if err := db.QueryRow("SELECT tenant_id FROM `" + table + "` LIMIT 1").Scan(&tenantID); err != nil {
			t.Fatalf("query %s tenant: %v", table, err)
		}
		if tenantID != 1 {
			t.Fatalf("%s tenant = %d, want 1", table, tenantID)
		}
	}
}

func TestBackfillTenantTenantRejectsDuplicateValuesWithoutPartialUpdate(t *testing.T) {
	db := newLegacyTenantTestDB(t)
	defer db.Close()

	if _, err := db.Exec("INSERT INTO roles(tenant_id, name) VALUES (11, 'admin'), (12, 'admin')"); err != nil {
		t.Fatalf("seed duplicate roles: %v", err)
	}
	if err := backfillTenantScope(context.Background(), db, 1); err == nil {
		t.Fatal("backfillTenantScope() error = nil")
	}
	var unchanged int
	if err := db.QueryRow("SELECT COUNT(*) FROM roles WHERE tenant_id IN (11, 12)").Scan(&unchanged); err != nil {
		t.Fatalf("query unchanged roles: %v", err)
	}
	if unchanged != 2 {
		t.Fatalf("unchanged roles = %d, want 2", unchanged)
	}
}

func newLegacyTenantTestDB(t *testing.T) *stdsql.DB {
	t.Helper()
	db, err := stdsql.Open("sqlite", filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	for _, statement := range []string{
		"CREATE TABLE users (tenant_id INTEGER NOT NULL, name TEXT, email TEXT, phone TEXT)",
		"CREATE TABLE roles (tenant_id INTEGER NOT NULL, name TEXT)",
		"CREATE TABLE posts (tenant_id INTEGER NOT NULL, name TEXT)",
		"CREATE TABLE depts (tenant_id INTEGER NOT NULL, name TEXT)",
		"CREATE TABLE projects (tenant_id INTEGER NOT NULL, name TEXT, code TEXT)",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("create legacy table: %v", err)
		}
	}
	return db
}
