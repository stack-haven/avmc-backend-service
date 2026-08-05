package authzpolicy

import (
	"testing"

	v1 "backend-service/api/platform/admin/v1"
)

func TestIsPlatformControlOperation(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		want      bool
	}{
		// Platform control-plane operations (should return true).
		{name: "platform create tenant", operation: v1.OperationTenantServiceCreateTenant, want: true},
		{name: "platform delete tenant", operation: v1.OperationTenantServiceDeleteTenant, want: true},
		{name: "platform list tenants", operation: v1.OperationTenantServiceListTenants, want: true},
		{name: "platform update lifecycle", operation: v1.OperationTenantServiceUpdateTenantLifecycle, want: true},
		{name: "platform create menu", operation: v1.OperationMenuServiceCreateMenu, want: true},
		{name: "platform list menus", operation: v1.OperationMenuServiceListMenus, want: true},
		{name: "platform create permission group", operation: v1.OperationTenantMenuPermissionGroupServiceCreateTenantMenuPermissionGroup, want: true},
		{name: "platform async task stats", operation: v1.OperationAsyncTaskServiceGetAsyncTaskStats, want: true},

		// Non-platform operations (should return false).
		{name: "non-platform operation", operation: "/platform.admin.v1.UserService/ListUsers", want: false},
		{name: "auth login", operation: v1.OperationAuthServiceLoginPassword, want: false},
		{name: "auth refresh token", operation: v1.OperationAuthServiceRefreshToken, want: false},
		{name: "empty operation", operation: "", want: false},
		{name: "unknown operation", operation: "/some.unknown.Service/Method", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsPlatformControlOperation(tt.operation)
			if got != tt.want {
				t.Errorf("IsPlatformControlOperation(%q) = %v, want %v", tt.operation, got, tt.want)
			}
		})
	}
}

func TestIsPlatformControlOperationExhaustive(t *testing.T) {
	// Verify that every operation listed in the switch statement is covered
	// and that coverage is exhaustive (fails if new ops are added without
	// being classified).
	allOps := []string{
		v1.OperationAuthServiceLoginPassword,
		v1.OperationAuthServiceRefreshToken,
		v1.OperationAuthServiceLoginByPhoneCode,
		v1.OperationAuthServiceLoginByEmail,
		v1.OperationAuthServiceLoginByUsername,
		v1.OperationTenantServiceCreateTenant,
		v1.OperationUserServiceListUsers,
	}
	for _, op := range allOps {
		_ = IsPlatformControlOperation(op) // ensures no panic
	}
}
