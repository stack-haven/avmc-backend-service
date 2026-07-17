package authzpolicy

import (
	"context"
	"testing"

	v1 "backend-service/api/platform/admin/v1"
	"backend-service/pkg/auth/authz"
	"backend-service/pkg/auth/authz/casbin"
)

func TestSyncSuperAdminAllowsHTTPAndGRPCActions(t *testing.T) {
	ctx := context.Background()
	provider := casbin.NewProvider()
	authorizer, err := provider.NewAuthorizer(ctx)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	role := authz.Subject("super_admin")
	tenant := authz.Tenant("1")
	if err := SyncSuperAdmin(ctx, authorizer, role, tenant, []authz.Subject{"1"}); err != nil {
		t.Fatalf("sync policies: %v", err)
	}

	if ok, err := authorizer.Enforce(ctx, "1", authz.Object(v1.OperationUserServiceListUsers), "GET", tenant); err != nil || !ok {
		t.Fatalf("http enforce = %v, %v", ok, err)
	}
	if ok, err := authorizer.Enforce(ctx, "1", authz.Object(v1.OperationUserServiceListUsers), "ListUsers", tenant); err != nil || !ok {
		t.Fatalf("grpc enforce = %v, %v", ok, err)
	}
	if ok, err := authorizer.Enforce(ctx, "1", authz.Object(v1.OperationTenantServiceListTenants), "GET", tenant); err == nil || ok {
		t.Fatalf("tenant admin unexpectedly received platform control access: %v, %v", ok, err)
	}
	if ok, err := authorizer.Enforce(ctx, "2", authz.Object(v1.OperationUserServiceListUsers), "GET", tenant); err == nil || ok {
		t.Fatalf("unexpected access for user without role: %v, %v", ok, err)
	}
}

func TestSyncPlatformAdminIncludesControlPlaneOperations(t *testing.T) {
	ctx := context.Background()
	provider := casbin.NewProvider()
	authorizer, err := provider.NewAuthorizer(ctx)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}
	if err := SyncPlatformAdmin(ctx, authorizer, "platform_admin", "1", []authz.Subject{"1"}); err != nil {
		t.Fatalf("sync platform policies: %v", err)
	}
	if ok, err := authorizer.Enforce(ctx, "1", authz.Object(v1.OperationTenantServiceListTenants), "GET", "1"); err != nil || !ok {
		t.Fatalf("platform http enforce = %v, %v", ok, err)
	}
	if ok, err := authorizer.Enforce(ctx, "1", authz.Object(v1.OperationMenuPermissionGroupServicePublishMenuPermissionGroupVersion), "PublishMenuPermissionGroupVersion", "1"); err != nil || !ok {
		t.Fatalf("platform grpc enforce = %v, %v", ok, err)
	}
}

func TestSyncSuperAdminRemovesStalePlatformPolicies(t *testing.T) {
	ctx := context.Background()
	provider := casbin.NewProvider()
	authorizer, err := provider.NewAuthorizer(ctx)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}
	if err := SyncPlatformAdmin(ctx, authorizer, "super_admin", "2", []authz.Subject{"2"}); err != nil {
		t.Fatalf("sync platform policies: %v", err)
	}
	if err := SyncSuperAdmin(ctx, authorizer, "super_admin", "2", []authz.Subject{"2"}); err != nil {
		t.Fatalf("downgrade to tenant policies: %v", err)
	}
	if ok, err := authorizer.Enforce(ctx, "2", authz.Object(v1.OperationTenantServiceListTenants), "GET", "2"); err == nil || ok {
		t.Fatalf("stale platform policy remained after tenant sync: %v, %v", ok, err)
	}
}

func TestSyncAdminRemovesStaleUserMembership(t *testing.T) {
	ctx := context.Background()
	provider := casbin.NewProvider()
	authorizer, err := provider.NewAuthorizer(ctx)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}
	if err := SyncSuperAdmin(ctx, authorizer, "super_admin", "1", []authz.Subject{"1", "2"}); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	if err := SyncSuperAdmin(ctx, authorizer, "super_admin", "1", []authz.Subject{"1"}); err != nil {
		t.Fatalf("second sync: %v", err)
	}
	if ok, err := authorizer.Enforce(ctx, "2", authz.Object(v1.OperationUserServiceListUsers), "GET", "1"); err == nil || ok {
		t.Fatalf("stale user retained admin membership: %v, %v", ok, err)
	}
}

func TestSetAdminMembershipGrantsAndRevokesWithoutChangingOtherUsers(t *testing.T) {
	ctx := context.Background()
	provider := casbin.NewProvider()
	authorizer, err := provider.NewAuthorizer(ctx)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}
	for _, user := range []authz.Subject{"1", "2"} {
		if err := SetAdminMembership(ctx, authorizer, "1", user, false, true); err != nil {
			t.Fatalf("grant user %s: %v", user, err)
		}
	}
	if err := SetAdminMembership(ctx, authorizer, "1", "1", false, false); err != nil {
		t.Fatalf("revoke user 1: %v", err)
	}
	if ok, err := authorizer.Enforce(ctx, "1", authz.Object(v1.OperationUserServiceListUsers), "GET", "1"); err == nil || ok {
		t.Fatalf("revoked user retained access: %v, %v", ok, err)
	}
	if ok, err := authorizer.Enforce(ctx, "2", authz.Object(v1.OperationUserServiceListUsers), "GET", "1"); err != nil || !ok {
		t.Fatalf("other admin lost access: %v, %v", ok, err)
	}
}

func TestCurrentTenantMenusRemainTenantOperation(t *testing.T) {
	if IsPlatformControlOperation(v1.OperationTenantPermissionServiceGetCurrentTenantEffectiveMenus) {
		t.Fatal("current tenant effective menus must remain available to tenant identities")
	}
	if IsPlatformControlOperation(v1.OperationTenantPermissionServiceGetCurrentTenantCapabilities) {
		t.Fatal("current tenant capabilities must remain available to tenant identities")
	}
	for _, operation := range []string{
		v1.OperationTenantPermissionServiceCheckCurrentTenantResourceQuota,
		v1.OperationTenantPermissionServiceConsumeCurrentTenantResourceQuota,
		v1.OperationTenantPermissionServiceListCurrentTenantResourceQuotas,
		v1.OperationTenantPermissionServiceReleaseCurrentTenantResourceQuota,
	} {
		if IsPlatformControlOperation(operation) {
			t.Fatalf("current tenant resource quota operation must remain available to tenant identities: %s", operation)
		}
	}
}

func TestMatchProtectedOperation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		object authz.Object
		action authz.Action
		want   bool
	}{
		{
			name:   "http action",
			object: authz.Object(v1.OperationUserServiceListUsers),
			action: "GET",
			want:   true,
		},
		{
			name:   "grpc action",
			object: authz.Object(v1.OperationUserServiceListUsers),
			action: "ListUsers",
			want:   true,
		},
		{
			name:   "wrong action",
			object: authz.Object(v1.OperationUserServiceListUsers),
			action: "POST",
			want:   false,
		},
		{
			name:   "unknown object",
			object: "unknown.Operation",
			action: "GET",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchProtectedOperation(tt.object, tt.action); got != tt.want {
				t.Fatalf("MatchProtectedOperation(%q, %q) = %v, want %v", tt.object, tt.action, got, tt.want)
			}
		})
	}
}

func TestAuthenticatedSelfServiceOperations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		object authz.Object
		action authz.Action
		want   bool
	}{
		{name: "profile http", object: authz.Object(v1.OperationAuthServiceProfile), action: "GET", want: true},
		{name: "profile grpc", object: authz.Object(v1.OperationAuthServiceProfile), action: "Profile", want: true},
		{name: "logout http", object: authz.Object(v1.OperationAuthServiceLogout), action: "POST", want: true},
		{name: "logout grpc", object: authz.Object(v1.OperationAuthServiceLogout), action: "Logout", want: true},
		{name: "my sessions", object: authz.Object(v1.OperationSessionServiceListMySessions), action: "GET", want: true},
		{name: "current tenant capabilities", object: authz.Object(v1.OperationTenantPermissionServiceGetCurrentTenantCapabilities), action: "GET", want: true},
		{name: "current tenant capabilities grpc", object: authz.Object(v1.OperationTenantPermissionServiceGetCurrentTenantCapabilities), action: "GetCurrentTenantCapabilities", want: true},
		{name: "current tenant quota list", object: authz.Object(v1.OperationTenantPermissionServiceListCurrentTenantResourceQuotas), action: "GET", want: true},
		{name: "current tenant quota check", object: authz.Object(v1.OperationTenantPermissionServiceCheckCurrentTenantResourceQuota), action: "GET", want: true},
		{name: "current tenant quota consume", object: authz.Object(v1.OperationTenantPermissionServiceConsumeCurrentTenantResourceQuota), action: "POST", want: true},
		{name: "current tenant quota consume grpc", object: authz.Object(v1.OperationTenantPermissionServiceConsumeCurrentTenantResourceQuota), action: "ConsumeCurrentTenantResourceQuota", want: true},
		{name: "current tenant quota release", object: authz.Object(v1.OperationTenantPermissionServiceReleaseCurrentTenantResourceQuota), action: "POST", want: true},
		{name: "wrong action", object: authz.Object(v1.OperationAuthServiceProfile), action: "POST", want: false},
		{name: "ordinary protected operation", object: authz.Object(v1.OperationUserServiceListUsers), action: "GET", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAuthenticatedSelfServiceOperation(tt.object, tt.action); got != tt.want {
				t.Fatalf("IsAuthenticatedSelfServiceOperation(%q, %q) = %v, want %v", tt.object, tt.action, got, tt.want)
			}
		})
	}
}

func TestParameterControlPlaneClassification(t *testing.T) {
	if !IsPlatformControlOperation(v1.OperationParameterServiceListParameterDefinitions) {
		t.Fatal("parameter definitions must be platform control-plane operations")
	}
	if !IsPlatformControlOperation(v1.OperationParameterServiceSetTenantParameter) {
		t.Fatal("cross-tenant parameter update must be a platform control-plane operation")
	}
	if IsPlatformControlOperation(v1.OperationParameterServiceSetCurrentTenantParameter) {
		t.Fatal("current tenant parameter update must remain a tenant data-plane operation")
	}
}

func TestConfigurationResourceBoundaryClassification(t *testing.T) {
	t.Parallel()

	tenantDictionaryOperations := []string{
		v1.OperationDictionaryServiceListDictionaryTypes,
		v1.OperationDictionaryServiceGetDictionaryType,
		v1.OperationDictionaryServiceCreateDictionaryType,
		v1.OperationDictionaryServiceUpdateDictionaryType,
		v1.OperationDictionaryServiceDeleteDictionaryType,
		v1.OperationDictionaryServiceListDictionaryItems,
		v1.OperationDictionaryServiceCreateDictionaryItem,
		v1.OperationDictionaryServiceUpdateDictionaryItem,
		v1.OperationDictionaryServiceDeleteDictionaryItem,
	}
	for _, operation := range tenantDictionaryOperations {
		if IsPlatformControlOperation(operation) {
			t.Fatalf("tenant dictionary operation must remain tenant data-plane: %s", operation)
		}
	}

	platformParameterOperations := []string{
		v1.OperationParameterServiceListParameterDefinitions,
		v1.OperationParameterServiceGetParameterDefinition,
		v1.OperationParameterServiceCreateParameterDefinition,
		v1.OperationParameterServiceUpdateParameterDefinition,
		v1.OperationParameterServiceDeleteParameterDefinition,
		v1.OperationParameterServiceListTenantParameters,
		v1.OperationParameterServiceSetTenantParameter,
		v1.OperationParameterServiceResetTenantParameter,
	}
	for _, operation := range platformParameterOperations {
		if !IsPlatformControlOperation(operation) {
			t.Fatalf("parameter control operation must remain platform control-plane: %s", operation)
		}
	}

	currentTenantParameterOperations := []string{
		v1.OperationParameterServiceListCurrentTenantParameters,
		v1.OperationParameterServiceSetCurrentTenantParameter,
		v1.OperationParameterServiceResetCurrentTenantParameter,
	}
	for _, operation := range currentTenantParameterOperations {
		if IsPlatformControlOperation(operation) {
			t.Fatalf("current tenant parameter operation must remain tenant data-plane: %s", operation)
		}
	}
}

func TestAsyncTaskOperationsRequirePlatformIdentity(t *testing.T) {
	for _, operation := range []string{
		v1.OperationAsyncTaskServiceListAsyncTasks,
		v1.OperationAsyncTaskServiceGetAsyncTaskStats,
		v1.OperationAsyncTaskServiceGetAsyncTask,
		v1.OperationAsyncTaskServiceCancelAsyncTask,
		v1.OperationAsyncTaskServiceRetryAsyncTask,
	} {
		if !IsPlatformControlOperation(operation) {
			t.Fatalf("async task operation must be platform control-plane: %s", operation)
		}
	}
}

func TestPoliciesForRoleDoNotIncludeLoginWhitelist(t *testing.T) {
	policies := PoliciesForRole("super_admin", "1")
	for _, policy := range policies {
		switch string(policy.Object) {
		case v1.OperationAuthServiceLoginPassword,
			v1.OperationAuthServiceLoginByEmail,
			v1.OperationAuthServiceLoginByUsername,
			v1.OperationAuthServiceLoginByPhoneCode,
			v1.OperationAuthServiceRefreshToken:
			t.Fatalf("whitelisted operation should not require seeded policy: %s", policy.Object)
		}
	}
}
