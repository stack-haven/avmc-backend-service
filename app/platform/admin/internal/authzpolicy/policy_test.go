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
	if ok, err := authorizer.Enforce(ctx, "2", authz.Object(v1.OperationUserServiceListUsers), "GET", tenant); err == nil || ok {
		t.Fatalf("unexpected access for user without role: %v, %v", ok, err)
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
