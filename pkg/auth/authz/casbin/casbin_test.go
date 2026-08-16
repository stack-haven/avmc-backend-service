package casbin

import (
	"context"
	"testing"

	"backend-service/pkg/auth/authz"
)

func newTestAuthorizer(t *testing.T) authz.Authorizer {
	t.Helper()
	a, err := authz.NewAuthorizer("casbin", context.Background(),
		authz.WithAdapterType(authz.AdapterMemory),
	)
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}
	return a
}

func TestEnforceAllowsAddedPolicy(t *testing.T) {
	a := newTestAuthorizer(t)
	ctx := context.Background()

	_, err := a.AddPolicy(ctx, authz.Policy{
		Subject: "user1", Object: "/api/resource", Action: "GET",
		Tenant: "tenant1", Effect: authz.EffectAllow,
	})
	if err != nil {
		t.Fatalf("add policy: %v", err)
	}

	allowed, err := a.Enforce(ctx, "user1", "/api/resource", "GET", "tenant1")
	if err != nil {
		t.Fatalf("enforce: %v", err)
	}
	if !allowed {
		t.Fatal("expected allow")
	}
}

func TestEnforceDeniesMissingPolicy(t *testing.T) {
	a := newTestAuthorizer(t)
	ctx := context.Background()

	allowed, err := a.Enforce(ctx, "user1", "/api/resource", "GET", "tenant1")
	if err != nil {
		// 未授权时 Casbin Enforce 返回错误（permission denied）
		if !authz.IsAuthzError(err) {
			t.Fatalf("expected authz error, got %v", err)
		}
		return
	}
	if allowed {
		t.Fatal("expected deny for missing policy")
	}
}

func TestRemovePolicy(t *testing.T) {
	a := newTestAuthorizer(t)
	ctx := context.Background()

	p := authz.Policy{
		Subject: "user1", Object: "/api/resource", Action: "GET",
		Tenant: "tenant1", Effect: authz.EffectAllow,
	}
	a.AddPolicy(ctx, p)

	// 移除后应拒绝
	if _, err := a.RemovePolicy(ctx, p); err != nil {
		t.Fatalf("remove policy: %v", err)
	}
	allowed, err := a.Enforce(ctx, "user1", "/api/resource", "GET", "tenant1")
	if err == nil && allowed {
		t.Fatal("expected deny after remove")
	}
}

func TestRBACRoleInheritance(t *testing.T) {
	a := newTestAuthorizer(t)
	ctx := context.Background()

	// 角色 admin 有权限
	a.AddPolicy(ctx, authz.Policy{
		Subject: "admin", Object: "/api/admin", Action: "GET",
		Tenant: "tenant1", Effect: authz.EffectAllow,
	})
	// 用户 user1 属于 admin 角色
	if _, err := a.AddRoleForUser(ctx, "user1", "admin", "tenant1"); err != nil {
		t.Fatalf("add role for user: %v", err)
	}

	// user1 通过 admin 角色获得权限
	allowed, err := a.Enforce(ctx, "user1", "/api/admin", "GET", "tenant1")
	if err != nil {
		t.Fatalf("enforce: %v", err)
	}
	if !allowed {
		t.Fatal("expected allow via role inheritance")
	}

	// 获取用户角色
	roles, err := a.GetRolesForUser(ctx, "user1", "tenant1")
	if err != nil {
		t.Fatalf("get roles: %v", err)
	}
	if len(roles) != 1 || roles[0] != "admin" {
		t.Fatalf("roles = %v, want [admin]", roles)
	}
}

func TestBatchEnforce(t *testing.T) {
	a := newTestAuthorizer(t)
	ctx := context.Background()

	a.AddPolicy(ctx, authz.Policy{
		Subject: "user1", Object: "/api/resource", Action: "GET",
		Tenant: "tenant1", Effect: authz.EffectAllow,
	})

	results, err := a.BatchEnforce(ctx,
		[]authz.Subject{"user1", "user1"},
		[]authz.Object{"/api/resource", "/api/other"},
		[]authz.Action{"GET", "GET"},
		[]authz.Tenant{"tenant1", "tenant1"},
	)
	if err != nil {
		t.Fatalf("batch enforce: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results len = %d, want 2", len(results))
	}
	if !results[0] {
		t.Fatal("first should be allowed")
	}
	if results[1] {
		t.Fatal("second should be denied")
	}
}

func TestProviderRegistration(t *testing.T) {
	if _, ok := authz.GetProvider("casbin"); !ok {
		t.Fatal("casbin provider should be registered")
	}
	if _, err := authz.NewAuthorizer("unknown", context.Background()); err == nil {
		t.Fatal("expected error for unknown provider")
	}
}
