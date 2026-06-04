package data

import (
	"context"
	stdsql "database/sql"
	"path/filepath"
	"testing"
)

func TestBackfillTenantDomain(t *testing.T) {
	db := newLegacyTenantTestDB(t)
	defer db.Close()

	for _, statement := range []string{
		"INSERT INTO users(domain_id, name, email, phone) VALUES (11, 'admin', 'admin@example.com', '100')",
		"INSERT INTO roles(domain_id, name) VALUES (12, 'admin')",
		"INSERT INTO posts(domain_id, name) VALUES (13, 'developer')",
		"INSERT INTO depts(domain_id, name) VALUES (14, 'engineering')",
		"INSERT INTO projects(domain_id, name, code) VALUES (15, 'app', 'app')",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("seed legacy data: %v", err)
		}
	}

	if err := backfillTenantDomain(context.Background(), db, 1); err != nil {
		t.Fatalf("backfillTenantDomain: %v", err)
	}
	for table := range tenantUniqueColumns {
		var domainID uint32
		if err := db.QueryRow("SELECT domain_id FROM `" + table + "` LIMIT 1").Scan(&domainID); err != nil {
			t.Fatalf("query %s domain: %v", table, err)
		}
		if domainID != 1 {
			t.Fatalf("%s domain_id = %d, want 1", table, domainID)
		}
	}
}

func TestBackfillTenantDomainRejectsDuplicateValuesWithoutPartialUpdate(t *testing.T) {
	db := newLegacyTenantTestDB(t)
	defer db.Close()

	if _, err := db.Exec("INSERT INTO roles(domain_id, name) VALUES (11, 'admin'), (12, 'admin')"); err != nil {
		t.Fatalf("seed duplicate roles: %v", err)
	}
	if err := backfillTenantDomain(context.Background(), db, 1); err == nil {
		t.Fatal("backfillTenantDomain() error = nil")
	}
	var unchanged int
	if err := db.QueryRow("SELECT COUNT(*) FROM roles WHERE domain_id IN (11, 12)").Scan(&unchanged); err != nil {
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
		"CREATE TABLE users (domain_id INTEGER NOT NULL, name TEXT, email TEXT, phone TEXT)",
		"CREATE TABLE roles (domain_id INTEGER NOT NULL, name TEXT)",
		"CREATE TABLE posts (domain_id INTEGER NOT NULL, name TEXT)",
		"CREATE TABLE depts (domain_id INTEGER NOT NULL, name TEXT)",
		"CREATE TABLE projects (domain_id INTEGER NOT NULL, name TEXT, code TEXT)",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("create legacy table: %v", err)
		}
	}
	return db
}
