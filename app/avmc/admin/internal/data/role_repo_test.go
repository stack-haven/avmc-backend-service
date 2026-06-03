package data

import (
	"context"
	stdsql "database/sql"
	"io"
	"testing"

	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/avmc/admin/internal/data/ent/gen"
	"backend-service/app/avmc/admin/internal/data/ent/gen/enttest"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/schema"
	_ "github.com/glebarez/go-sqlite"
	"github.com/go-kratos/kratos/v2/log"
)

func TestRoleRepoSaveAndUpdateMenuIDs(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	defer client.Close()

	parent := client.Menu.Create().
		SetName("system").
		SetTitle("System").
		SetStatus(1).
		SetType(1).
		SaveX(ctx)
	child := client.Menu.Create().
		SetName("users").
		SetTitle("Users").
		SetStatus(1).
		SetType(2).
		SetParentID(parent.ID).
		SaveX(ctx)

	repo := NewRoleRepo(&Data{db: client}, log.NewStdLogger(io.Discard))
	role, err := repo.Save(ctx, &pbCore.Role{
		Name:    ptr("admin"),
		MenuIds: []uint32{parent.ID, child.ID},
	})
	if err != nil {
		t.Fatalf("save role: %v", err)
	}

	assertRoleMenuIDs(t, client, role.GetId(), []uint32{parent.ID, child.ID})

	_, err = repo.Update(ctx, &pbCore.Role{
		Id:      role.GetId(),
		Name:    ptr("admin"),
		MenuIds: []uint32{},
	})
	if err != nil {
		t.Fatalf("update role: %v", err)
	}
	assertRoleMenuIDs(t, client, role.GetId(), nil)
}

func newTestClient(t *testing.T) *gen.Client {
	t.Helper()
	db, err := stdsql.Open("sqlite", "file:admin_role_repo?mode=memory&cache=shared&_fk=1")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		t.Fatalf("enable sqlite foreign keys: %v", err)
	}
	drv := entsql.OpenDB(dialect.SQLite, db)
	return enttest.NewClient(
		t,
		enttest.WithOptions(gen.Driver(drv)),
		enttest.WithMigrateOptions(schema.WithForeignKeys(false)),
	)
}

func assertRoleMenuIDs(t *testing.T, client *gen.Client, roleID uint32, want []uint32) {
	t.Helper()
	got, err := client.Role.GetX(context.Background(), roleID).QueryMenus().IDs(context.Background())
	if err != nil {
		t.Fatalf("query role menu ids: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("menu ids len = %d, want %d; got=%v", len(got), len(want), got)
	}
	wantSet := make(map[uint32]struct{}, len(want))
	for _, id := range want {
		wantSet[id] = struct{}{}
	}
	for _, id := range got {
		if _, ok := wantSet[id]; !ok {
			t.Fatalf("unexpected menu id %d in %v", id, got)
		}
	}
}

func ptr[T any](v T) *T {
	return &v
}
