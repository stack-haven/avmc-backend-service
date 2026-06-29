package data

import (
	"io"
	"testing"

	"backend-service/api/common/enum"
	pbCore "backend-service/api/core/service/v1"

	"github.com/go-kratos/kratos/v2/log"
)

func TestTenantAndMenuPermissionGroupRelationsAreReturned(t *testing.T) {
	ctx := systemContext()
	client := newTestClient(t)
	defer client.Close()

	tenant := client.Tenant.Create().
		SetName("demo tenant").
		SetCode("demo-tenant").
		SetStatus(int32(enum.Status_STATUS_ENABLED)).
		SaveX(ctx)
	menuItem := client.Menu.Create().
		SetName("demo-menu").
		SetTitle("Demo Menu").
		SetStatus(int32(enum.Status_STATUS_ENABLED)).
		SaveX(ctx)
	group := client.MenuPermissionGroup.Create().
		SetName("demo group").
		SetCode("demo-group").
		SetStatus(int32(enum.Status_STATUS_ENABLED)).
		AddMenuIDs(menuItem.ID).
		SaveX(ctx)
	client.TenantPermissionGroup.Create().
		SetTenantID(tenant.ID).
		SetGroupID(group.ID).
		SetEnabled(true).
		SaveX(ctx)

	logger := log.NewStdLogger(io.Discard)
	tenantRepo := NewTenantRepo(&Data{db: client}, logger)
	tenants, err := tenantRepo.ListTenants(ctx)
	if err != nil {
		t.Fatalf("list tenants: %v", err)
	}
	if len(tenants) != 1 {
		t.Fatalf("tenant count = %d, want 1", len(tenants))
	}
	if got := tenants[0].GetGroupIds(); len(got) != 1 || got[0] != group.ID {
		t.Fatalf("tenant group ids = %v, want [%d]", got, group.ID)
	}
	if got := tenants[0].GetGroups(); len(got) != 1 || got[0].GetName() != "demo group" {
		t.Fatalf("tenant groups = %#v", got)
	}

	groupRepo := NewMenuPermissionGroupRepo(&Data{db: client}, logger)
	groups, err := groupRepo.ListMenuPermissionGroups(ctx)
	if err != nil {
		t.Fatalf("list groups: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("group count = %d, want 1", len(groups))
	}
	if got := groups[0].GetMenuIds(); len(got) != 1 || got[0] != menuItem.ID {
		t.Fatalf("group menu ids = %v, want [%d]", got, menuItem.ID)
	}
}

func TestMenuPermissionGroupUpdateRefreshesMenus(t *testing.T) {
	ctx := systemContext()
	client := newTestClient(t)
	defer client.Close()

	first := client.Menu.Create().SetName("first").SetTitle("First").SaveX(ctx)
	second := client.Menu.Create().SetName("second").SetTitle("Second").SaveX(ctx)
	group := client.MenuPermissionGroup.Create().
		SetName("demo group").
		SetCode("demo-group").
		SetStatus(int32(enum.Status_STATUS_ENABLED)).
		AddMenuIDs(first.ID).
		SaveX(ctx)

	repo := NewMenuPermissionGroupRepo(&Data{db: client}, log.NewStdLogger(io.Discard))
	status := enum.Status_STATUS_ENABLED
	updated, err := repo.Update(ctx, &pbCore.MenuPermissionGroup{
		Id:      group.ID,
		Name:    ptr("demo group"),
		Code:    ptr("demo-group"),
		Status:  &status,
		MenuIds: []uint32{second.ID},
	})
	if err != nil {
		t.Fatalf("update group: %v", err)
	}
	if got := updated.GetMenuIds(); len(got) != 1 || got[0] != second.ID {
		t.Fatalf("updated menu ids = %v, want [%d]", got, second.ID)
	}
}
