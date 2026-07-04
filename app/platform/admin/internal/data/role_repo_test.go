package data

import (
	stdsql "database/sql"
	"io"
	"testing"

	pbEnum "backend-service/api/common/enum"
	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/internal/data/ent/gen"
	"backend-service/app/platform/admin/internal/data/ent/gen/enttest"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/schema"
	_ "github.com/glebarez/go-sqlite"
	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

func TestRoleRepoSaveAndUpdateMenuIDs(t *testing.T) {
	ctx := tenantContext(1)
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
	seedTenantMenuPermissionGroup(t, client, 1, parent.ID, child.ID)

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

func TestRoleRepoEnforcesTenantIsolation(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()

	repo := NewRoleRepo(&Data{db: client}, log.NewStdLogger(io.Discard))
	first, err := repo.Save(tenantContext(1), &pbCore.Role{Name: ptr("operator")})
	if err != nil {
		t.Fatalf("save tenant one role: %v", err)
	}
	if _, err := repo.Save(tenantContext(2), &pbCore.Role{Name: ptr("operator")}); err != nil {
		t.Fatalf("save same role name in tenant two: %v", err)
	}
	if _, err := repo.FindByID(tenantContext(2), first.GetId()); !pb.IsRoleNotFound(err) {
		t.Fatalf("cross-tenant FindByID() error = %v", err)
	}
}

func TestRoleRepoRejectsInvalidMenuIDs(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()
	repo := NewRoleRepo(&Data{db: client}, log.NewStdLogger(io.Discard))

	if _, err := repo.Save(tenantContext(1), &pbCore.Role{
		Name:    ptr("operator"),
		MenuIds: []uint32{999},
	}); !pb.IsRolePermissionInvalid(err) {
		t.Fatalf("invalid menu IDs error = %v", err)
	}
}

func TestRoleRepoListReturnsMenuIDs(t *testing.T) {
	ctx := tenantContext(1)
	client := newTestClient(t)
	defer client.Close()
	menuItem := client.Menu.Create().SetName("listed-menu").SetTitle("Listed").SaveX(ctx)
	seedTenantMenuPermissionGroup(t, client, 1, menuItem.ID)
	repo := NewRoleRepo(&Data{db: client}, log.NewStdLogger(io.Discard))
	if _, err := repo.Save(ctx, &pbCore.Role{Name: ptr("listed-role"), MenuIds: []uint32{menuItem.ID}}); err != nil {
		t.Fatalf("save role: %v", err)
	}

	roles, err := repo.ListRoles(ctx)
	if err != nil {
		t.Fatalf("list roles: %v", err)
	}
	if len(roles) != 1 || len(roles[0].GetMenuIds()) != 1 || roles[0].GetMenuIds()[0] != menuItem.ID {
		t.Fatalf("roles = %#v", roles)
	}
}

func TestRoleRepoProtectsTenantAdminRole(t *testing.T) {
	ctx := tenantContext(1)
	client := newTestClient(t)
	defer client.Close()
	adminRole := client.Role.Create().
		SetName("tenant-admin").
		SetStatus(1).
		SetIsTenantAdmin(true).
		SaveX(ctx)
	repo := NewRoleRepo(&Data{db: client}, log.NewStdLogger(io.Discard))

	if _, err := repo.Update(ctx, &pbCore.Role{
		Id:     adminRole.ID,
		Name:   ptr("changed-admin"),
		Status: ptr(pbCoreStatusDisabled()),
	}); err == nil {
		t.Fatal("tenant admin role update unexpectedly succeeded")
	}
	if err := repo.Delete(ctx, adminRole.ID); err == nil {
		t.Fatal("tenant admin role delete unexpectedly succeeded")
	}
	stored := client.Role.GetX(ctx, adminRole.ID)
	if !stored.IsTenantAdmin || stored.Name == nil || *stored.Name != "tenant-admin" {
		t.Fatalf("tenant admin role changed: %+v", stored)
	}
}

func TestRoleRepoProtectsRoleAssignedToUsers(t *testing.T) {
	ctx := tenantContext(1)
	client := newTestClient(t)
	defer client.Close()
	assignedRole := client.Role.Create().
		SetName("assigned-role").
		SetStatus(1).
		SetDataScope(2).
		SaveX(ctx)
	client.User.Create().
		SetName("assigned-user").
		SetPassword("hashed-password").
		SetStatus(1).
		AddRoleIDs(assignedRole.ID).
		SaveX(ctx)
	repo := NewRoleRepo(&Data{db: client}, log.NewStdLogger(io.Discard))

	if err := repo.Delete(ctx, assignedRole.ID); !errors.IsConflict(err) {
		t.Fatalf("Delete() error = %v, want conflict", err)
	}
}

func pbCoreStatusDisabled() pbEnum.Status {
	return pbEnum.Status_STATUS_DISABLED
}

func seedTenantMenuPermissionGroup(t *testing.T, client *gen.Client, tenantID uint32, menuIDs ...uint32) {
	t.Helper()
	ctx := systemContext()
	client.Tenant.Create().
		SetID(tenantID).
		SetName("tenant").
		SetCode("tenant").
		SetStatus(1).
		SaveX(ctx)
	group := client.MenuPermissionGroup.Create().
		SetName("tenant-permission-group").
		SetCode("tenant-permission-group").
		SetStatus(1).
		AddMenuIDs(menuIDs...).
		SaveX(ctx)
	client.TenantPermissionGroup.Create().
		SetTenantID(tenantID).
		SetGroupID(group.ID).
		SetEnabled(true).
		SaveX(ctx)
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
	ctx := systemContext()
	got, err := client.Role.GetX(ctx, roleID).QueryMenus().IDs(ctx)
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
