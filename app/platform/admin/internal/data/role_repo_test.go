package data

import (
	"io"
	"testing"

	pbEnum "backend-service/api/common/enum"
	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"

	_ "github.com/glebarez/go-sqlite"
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

func TestRoleRepoExcludesDeletedMenusFromAuthorizations(t *testing.T) {
	ctx := tenantContext(1)
	client := newTestClient(t)
	defer client.Close()
	menuItem := client.Menu.Create().
		SetName("deleted-menu").
		SetTitle("Deleted").
		SetStatus(1).
		SaveX(ctx)
	seedTenantMenuPermissionGroup(t, client, 1, menuItem.ID)
	roleRepo := NewRoleRepo(&Data{db: client}, log.NewStdLogger(io.Discard))
	menuRepo := NewMenuRepo(&Data{db: client}, log.NewStdLogger(io.Discard))
	role, err := roleRepo.Save(ctx, &pbCore.Role{Name: ptr("menu-cleanup-role"), MenuIds: []uint32{menuItem.ID}})
	if err != nil {
		t.Fatalf("save role: %v", err)
	}

	if err := menuRepo.Delete(systemContext(), menuItem.ID); err != nil {
		t.Fatalf("delete menu: %v", err)
	}

	got, err := roleRepo.FindByID(ctx, role.GetId())
	if err != nil {
		t.Fatalf("find role: %v", err)
	}
	if len(got.GetMenuIds()) != 0 {
		t.Fatalf("FindByID() menu IDs = %v, want empty after menu deletion", got.GetMenuIds())
	}
	roles, err := roleRepo.ListRoles(ctx)
	if err != nil {
		t.Fatalf("list roles: %v", err)
	}
	if len(roles) != 1 || len(roles[0].GetMenuIds()) != 0 {
		t.Fatalf("ListRoles() roles = %#v, want deleted menu excluded", roles)
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

	disabled := pbEnum.Status_STATUS_DISABLED
	if _, err := repo.Update(ctx, &pbCore.Role{
		Id:     adminRole.ID,
		Name:   ptr("changed-admin"),
		Status: &disabled,
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
