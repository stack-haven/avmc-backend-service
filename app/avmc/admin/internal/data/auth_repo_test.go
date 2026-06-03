package data

import (
	"context"
	"io"
	"testing"

	pbCore "backend-service/api/core/service/v1"

	"github.com/go-kratos/kratos/v2/log"
)

func TestAuthRepoMenusFiltersDisabledRolesAndAddsAncestors(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	defer client.Close()

	parent := client.Menu.Create().
		SetName("system").
		SetTitle("System").
		SetStatus(1).
		SetType(int32(pbCore.MenuType_MENU_TYPE_CATALOG)).
		SaveX(ctx)
	child := client.Menu.Create().
		SetName("users").
		SetTitle("Users").
		SetStatus(1).
		SetType(int32(pbCore.MenuType_MENU_TYPE_MENU)).
		SetParentID(parent.ID).
		SetAuthCode("system:user:list").
		SaveX(ctx)
	disabledMenu := client.Menu.Create().
		SetName("disabled-role-menu").
		SetTitle("Disabled Role Menu").
		SetStatus(1).
		SetType(int32(pbCore.MenuType_MENU_TYPE_MENU)).
		SetAuthCode("system:disabled:list").
		SaveX(ctx)

	enabledRole := client.Role.Create().
		SetName("enabled_role").
		SetDefaultRouter("/").
		SetDataScope(1).
		SetMenuCheckStrictly(1).
		SetDeptCheckStrictly(1).
		SetStatus(1).
		SetDomainID(1).
		AddMenuIDs(child.ID).
		SaveX(ctx)
	disabledRole := client.Role.Create().
		SetName("disabled_role").
		SetDefaultRouter("/").
		SetDataScope(1).
		SetMenuCheckStrictly(1).
		SetDeptCheckStrictly(1).
		SetStatus(2).
		SetDomainID(1).
		AddMenuIDs(disabledMenu.ID).
		SaveX(ctx)
	otherDomainRole := client.Role.Create().
		SetName("other_domain_role").
		SetDefaultRouter("/").
		SetDataScope(1).
		SetMenuCheckStrictly(1).
		SetDeptCheckStrictly(1).
		SetStatus(1).
		SetDomainID(2).
		AddMenuIDs(disabledMenu.ID).
		SaveX(ctx)
	user := client.User.Create().
		SetName("tester").
		SetPassword("secret1").
		SetStatus(1).
		SetDomainID(1).
		AddRoleIDs(enabledRole.ID, disabledRole.ID, otherDomainRole.ID).
		SaveX(ctx)

	logger := log.NewStdLogger(io.Discard)
	repo := &authRepo{
		data: &Data{db: client},
		log:  log.NewHelper(logger),
		mr:   NewMenuRepo(&Data{db: client}, logger).(*menuRepo),
	}

	codes, err := repo.Codes(ctx, user.ID)
	if err != nil {
		t.Fatalf("codes: %v", err)
	}
	if len(codes) != 1 || codes[0] != "system:user:list" {
		t.Fatalf("codes = %#v", codes)
	}

	menus, err := repo.Menus(ctx, user.ID)
	if err != nil {
		t.Fatalf("menus: %v", err)
	}
	if len(menus) != 1 {
		t.Fatalf("root menu len = %d, menus=%#v", len(menus), menus)
	}
	if menus[0].GetId() != parent.ID {
		t.Fatalf("root id = %d, want parent %d", menus[0].GetId(), parent.ID)
	}
	if len(menus[0].Children) != 1 || menus[0].Children[0].GetId() != child.ID {
		t.Fatalf("children = %#v, want child %d", menus[0].Children, child.ID)
	}
}
