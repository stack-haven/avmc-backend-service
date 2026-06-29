package data

import (
	pbEnum "backend-service/api/common/enum"
	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/app/platform/admin/internal/data/ent/gen/tenant"
	"io"
	"strings"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
)

func TestTenantRepoProvisionCreatesCompleteTenant(t *testing.T) {
	ctx := systemContext()
	client := newTestClient(t)
	defer client.Close()

	menuItem := client.Menu.Create().
		SetName("tenant-dashboard").
		SetTitle("Tenant Dashboard").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		SaveX(ctx)
	group := client.MenuPermissionGroup.Create().
		SetName("standard package").
		SetCode("standard-package").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		AddMenuIDs(menuItem.ID).
		SaveX(ctx)

	repo := NewTenantRepo(&Data{db: client}, log.NewStdLogger(io.Discard))
	status := pbEnum.Status_STATUS_ENABLED
	result, err := repo.Provision(ctx, &biz.TenantProvisioning{
		Tenant: &pbCore.Tenant{
			Name:     ptr("Acme"),
			Code:     ptr("acme"),
			Status:   &status,
			GroupIds: []uint32{group.ID},
		},
		AdminUsername:     "acme_admin",
		AdminPasswordHash: "hashed-password",
		AdminRealname:     "Acme Administrator",
		AdminEmail:        "admin@acme.example",
	})
	if err != nil {
		t.Fatalf("provision tenant: %v", err)
	}
	if result.Tenant.GetLifecycleStatus() != pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_ACTIVE {
		t.Fatalf("lifecycle = %v, want ACTIVE", result.Tenant.GetLifecycleStatus())
	}
	if result.AdminUserID == 0 || result.AdminRoleID == 0 || result.RootDeptID == 0 {
		t.Fatalf("incomplete provisioning result: %#v", result)
	}
	roleMenus := client.Role.GetX(ctx, result.AdminRoleID).QueryMenus().IDsX(ctx)
	if len(roleMenus) != 1 || roleMenus[0] != menuItem.ID {
		t.Fatalf("admin role menus = %v, want [%d]", roleMenus, menuItem.ID)
	}
	userRoles := client.User.GetX(ctx, result.AdminUserID).QueryRoles().IDsX(ctx)
	if len(userRoles) != 1 || userRoles[0] != result.AdminRoleID {
		t.Fatalf("admin user roles = %v, want [%d]", userRoles, result.AdminRoleID)
	}
	rootDept := client.Dept.GetX(ctx, result.RootDeptID)
	if rootDept.TenantID != result.Tenant.GetId() || rootDept.LeaderID == nil || *rootDept.LeaderID != result.AdminUserID {
		t.Fatalf("root department not initialized: %#v", rootDept)
	}
}

func TestTenantRepoProvisionRollsBackOnInitializationFailure(t *testing.T) {
	ctx := systemContext()
	client := newTestClient(t)
	defer client.Close()

	menuItem := client.Menu.Create().SetName("rollback-menu").SetTitle("Rollback").SetStatus(1).SaveX(ctx)
	group := client.MenuPermissionGroup.Create().
		SetName("rollback package").
		SetCode("rollback-package").
		SetStatus(1).
		AddMenuIDs(menuItem.ID).
		SaveX(ctx)
	repo := NewTenantRepo(&Data{db: client}, log.NewStdLogger(io.Discard))
	status := pbEnum.Status_STATUS_ENABLED
	_, err := repo.Provision(ctx, &biz.TenantProvisioning{
		Tenant: &pbCore.Tenant{
			Name:     ptr("Rollback Tenant"),
			Code:     ptr("rollback-tenant"),
			Status:   &status,
			GroupIds: []uint32{group.ID},
		},
		AdminUsername:     "rollback_admin",
		AdminPasswordHash: "hashed-password",
		AdminRealname:     strings.Repeat("x", 51),
	})
	if err == nil {
		t.Fatal("provision tenant error = nil, want initialization failure")
	}
	exists, queryErr := client.Tenant.Query().Where(tenant.CodeEQ("rollback-tenant")).Exist(ctx)
	if queryErr != nil {
		t.Fatalf("query rolled back tenant: %v", queryErr)
	}
	if exists {
		t.Fatal("tenant remains after provisioning transaction failure")
	}
}
