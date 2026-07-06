package data

import (
	"context"
	"strconv"
	"testing"

	pbEnum "backend-service/api/common/enum"
	pbAdmin "backend-service/api/platform/admin/v1"
	"backend-service/pkg/auth/authz"
	"backend-service/pkg/auth/authz/casbin"
)

func TestTenantRoleAuthorizerUsesRoleMenuPermission(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()

	ctx := tenantContext(1)
	permission := client.Menu.Create().
		SetName("user-list-permission").
		SetTitle("User list").
		SetAuthCode(pbAdmin.OperationUserServiceListUsers).
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		SaveX(systemContext())
	group := client.MenuPermissionGroup.Create().
		SetName("operator package").
		SetCode("operator-package").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		AddMenuIDs(permission.ID).
		SaveX(systemContext())
	binding := client.TenantPermissionGroup.Create().
		SetTenantID(1).
		SetGroupID(group.ID).
		SetEnabled(true).
		SaveX(systemContext())
	tenantRole := client.Role.Create().
		SetName("operator").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		AddMenuIDs(permission.ID).
		SaveX(ctx)
	tenantUser := client.User.Create().
		SetName("operator").
		SetPassword("hashed").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		AddRoleIDs(tenantRole.ID).
		SaveX(ctx)

	provider := casbin.NewProvider()
	base, err := provider.NewAuthorizer(context.Background(), authz.WithAdapterType(authz.AdapterMemory))
	if err != nil {
		t.Fatalf("create base authorizer: %v", err)
	}
	authorizer := newTenantRoleAuthorizer(base, client)
	subject := authz.Subject(strconv.FormatUint(uint64(tenantUser.ID), 10))

	allowed, err := authorizer.Enforce(ctx, subject, pbAdmin.OperationUserServiceListUsers, "GET", "1")
	if err != nil || !allowed {
		t.Fatalf("role menu permission denied: allowed=%v err=%v", allowed, err)
	}
	allowed, err = authorizer.Enforce(ctx, subject, pbAdmin.OperationUserServiceDeleteUser, "DELETE", "1")
	if err != nil {
		t.Fatalf("check denied permission: %v", err)
	}
	if allowed {
		t.Fatal("operation without a role menu permission was allowed")
	}
	allowed, err = authorizer.Enforce(ctx, subject, pbAdmin.OperationUserServiceListUsers, "GET", "2")
	if err != nil {
		t.Fatalf("check cross-tenant permission: %v", err)
	}
	if allowed {
		t.Fatal("cross-tenant role permission was allowed")
	}
	client.TenantPermissionGroup.DeleteOne(binding).ExecX(systemContext())
	allowed, err = authorizer.Enforce(ctx, subject, pbAdmin.OperationUserServiceListUsers, "GET", "1")
	if err != nil {
		t.Fatalf("check permission after package removal: %v", err)
	}
	if allowed {
		t.Fatal("role permission outside the tenant package was allowed")
	}
}

func TestTenantRoleAuthorizerAllowsAuthenticatedSelfService(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()
	ctx := tenantContext(1)
	tenantUser := client.User.Create().
		SetName("member").
		SetPassword("hashed").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		SaveX(ctx)

	provider := casbin.NewProvider()
	base, err := provider.NewAuthorizer(context.Background(), authz.WithAdapterType(authz.AdapterMemory))
	if err != nil {
		t.Fatalf("create base authorizer: %v", err)
	}
	authorizer := newTenantRoleAuthorizer(base, client)
	allowed, err := authorizer.Enforce(
		ctx,
		authz.Subject(strconv.FormatUint(uint64(tenantUser.ID), 10)),
		pbAdmin.OperationAuthServiceMenus,
		"GET",
		"1",
	)
	if err != nil || !allowed {
		t.Fatalf("authenticated self-service denied: allowed=%v err=%v", allowed, err)
	}
}
