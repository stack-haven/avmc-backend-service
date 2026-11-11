package data

import (
	"context"
	"testing"

	"backend-service/app/platform/service/internal/data/ent/viewer"
)

// TestEntPrivacyTenantIsolation 验证 Ent Privacy 租户隔离规则的真实行为。
//
// 该用例覆盖的是安全不变量本身（而不是某个仓储的间接效果），因此使用
// 进程内 SQLite（复用 newTestClient）直接驱动 Ent 客户端：
//  1. 系统上下文可跨租户读取；
//  2. 租户上下文只能读到本租户数据；
//  3. 缺少租户上下文时查询被拒绝（fail-closed）；
//  4. 创建时 tenant_id 由规则注入，无需要调用方显式传入；
//  5. 跨租户 Update 被租户谓词过滤掉（不会误改他人数据）。
//
// 历史说明：本文件原为 ent_privacy_integration_test.go，带
// `//go:build integration` 标签且存在类型错误，从未被任何 CI 执行。
func TestEntPrivacyTenantIsolation(t *testing.T) {
	client := newTestClient(t)
	sysCtx := viewer.NewSystemContext(context.Background())

	t1 := client.Tenant.Create().SetName("Tenant A").SetCode("ta").SetLifecycleStatus(2).SaveX(sysCtx)
	t2 := client.Tenant.Create().SetName("Tenant B").SetCode("tb").SetLifecycleStatus(2).SaveX(sysCtx)

	u1 := client.User.Create().
		SetTenantID(t1.ID).SetName("user_a").SetPassword("hash-user-a").SetStatus(1).
		SaveX(sysCtx)
	u2 := client.User.Create().
		SetTenantID(t2.ID).SetName("user_b").SetPassword("hash-user-b").SetStatus(1).
		SaveX(sysCtx)

	t.Run("system context sees all users", func(t *testing.T) {
		count, err := client.User.Query().Count(sysCtx)
		if err != nil {
			t.Fatalf("count users: %v", err)
		}
		if count != 2 {
			t.Fatalf("system context: expected 2 users, got %d", count)
		}
	})

	t.Run("tenant-scoped context sees only own users", func(t *testing.T) {
		ctxA := viewer.NewTenantContext(context.Background(), t1.ID)
		users, err := client.User.Query().All(ctxA)
		if err != nil {
			t.Fatalf("query users for tenant A: %v", err)
		}
		if len(users) != 1 {
			t.Fatalf("tenant A: expected 1 user, got %d", len(users))
		}
		if users[0].ID != u1.ID {
			t.Fatalf("tenant A: expected user %d, got %d", u1.ID, users[0].ID)
		}
		if name := users[0].Name; name == nil || *name != "user_a" {
			t.Fatalf("tenant A: expected name user_a, got %v", users[0].Name)
		}
	})

	t.Run("missing tenant context is denied", func(t *testing.T) {
		if _, err := client.User.Query().All(context.Background()); err == nil {
			t.Fatal("expected error when no tenant context is present")
		}
	})

	t.Run("create injects tenant_id from context", func(t *testing.T) {
		ctxA := viewer.NewTenantContext(context.Background(), t1.ID)
		created := client.User.Create().
			SetName("user_auto").SetPassword("hash-auto").SetStatus(1).
			SaveX(ctxA)
		if created.TenantID != t1.ID {
			t.Fatalf("expected injected tenant_id %d, got %d", t1.ID, created.TenantID)
		}
	})

	t.Run("cross-tenant update is filtered out", func(t *testing.T) {
		ctxA := viewer.NewTenantContext(context.Background(), t1.ID)
		err := client.User.UpdateOneID(u2.ID).SetName("hijacked").Exec(ctxA)
		if err == nil {
			t.Fatal("expected cross-tenant update to be rejected")
		}
		reloaded := client.User.GetX(sysCtx, u2.ID)
		if name := reloaded.Name; name == nil || *name != "user_b" {
			t.Fatalf("tenant B user was modified: %v", reloaded.Name)
		}
	})
}
