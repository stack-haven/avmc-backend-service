package data

import (
	"context"
	"testing"

	"backend-service/app/ai/service/internal/data/ent/gen/chatmessage"
)

// TestChatTenantIsolation 验证 ai 服务的租户隔离真实生效。
//
// 背景（B4）：ai 此前只有 domain_id（随机雪花默认值，创建不赋值、查询不过滤），
// 任何租户都能读到全部对话；chat.name 还是全局唯一，不同租户会互相占用名字。
// 本用例锁定修复后的不变量。
func TestChatTenantIsolation(t *testing.T) {
	client, rawDB := newTestClientWithDB(t)
	sysCtx := systemContext()

	const tenantA, tenantB = uint32(1), uint32(2)

	chatA := client.Chat.Create().
		SetTenantID(tenantA).SetName("chat_a").SetChannel("web").
		SaveX(sysCtx)
	chatB := client.Chat.Create().
		SetTenantID(tenantB).SetName("chat_b").SetChannel("web").
		SaveX(sysCtx)
	client.ChatMessage.Create().
		SetTenantID(tenantA).SetChatID(chatA.ID).SetContent("hello-a").SetRole(chatmessage.RoleUser).
		SaveX(sysCtx)
	client.ChatMessage.Create().
		SetTenantID(tenantB).SetChatID(chatB.ID).SetContent("hello-b").SetRole(chatmessage.RoleUser).
		SaveX(sysCtx)

	t.Run("系统上下文可见全部", func(t *testing.T) {
		if n := client.Chat.Query().CountX(sysCtx); n != 2 {
			t.Fatalf("expected 2 chats, got %d", n)
		}
	})

	t.Run("租户上下文只见本租户", func(t *testing.T) {
		ctxA := tenantContext(tenantA)
		chats := client.Chat.Query().AllX(ctxA)
		if len(chats) != 1 || chats[0].ID != chatA.ID {
			t.Fatalf("tenant A 应只看到 %d，实际 %d 条", chatA.ID, len(chats))
		}
		msgs := client.ChatMessage.Query().AllX(ctxA)
		if len(msgs) != 1 || msgs[0].Content == nil || *msgs[0].Content != "hello-a" {
			t.Fatalf("tenant A 看到了其他租户的消息：%d 条", len(msgs))
		}
	})

	t.Run("缺少租户上下文被拒绝", func(t *testing.T) {
		if _, err := client.Chat.Query().All(context.Background()); err == nil {
			t.Fatal("expected denial without tenant context")
		}
	})

	t.Run("创建时注入 tenant_id", func(t *testing.T) {
		created := client.Chat.Create().
			SetName("chat_auto").SetChannel("web").
			SaveX(tenantContext(tenantA))
		if created.TenantID != tenantA {
			t.Fatalf("expected injected tenant %d, got %d", tenantA, created.TenantID)
		}
	})

	t.Run("显式传入他人 tenant_id 被拒绝", func(t *testing.T) {
		if _, err := client.Chat.Create().
			SetTenantID(tenantB).SetName("chat_hijack").SetChannel("web").
			Save(tenantContext(tenantA)); err == nil {
			t.Fatal("expected rejection when tenant_id mismatches the context")
		}
	})

	t.Run("对话名在租户内唯一、跨租户可重名", func(t *testing.T) {
		// tenant B 已有 chat_b；tenant A 使用同名应成功（复合唯一键按租户隔离）
		if _, err := client.Chat.Create().
			SetName("chat_b").SetChannel("web").
			Save(tenantContext(tenantA)); err != nil {
			t.Fatalf("跨租户同名应允许：%v", err)
		}
		// 同租户内重名应失败
		if _, err := client.Chat.Create().
			SetName("chat_b").SetChannel("web").
			Save(tenantContext(tenantA)); err == nil {
			t.Fatal("同租户内重名应被拒绝")
		}
	})

	t.Run("AssignLegacyTenant 只收敛 tenant_id=0 的历史行", func(t *testing.T) {
		// 模拟历史行：B4 之前没有 tenant_id 列，ALTER 加列时数据库会以 0 填充。
		// Ent 的 Positive() 校验器不允许写 0，所以这里用底层 SQL 还原真实形态。
		legacy := client.Chat.Create().
			SetTenantID(tenantB).SetName("chat_legacy").SetChannel("web").
			SaveX(sysCtx)
		if _, err := rawDB.Exec("UPDATE ai_chats SET tenant_id = 0 WHERE id = ?", legacy.ID); err != nil {
			t.Fatalf("simulate legacy row: %v", err)
		}
		if got, err := CountUnownedChats(sysCtx, client); err != nil || got < 1 {
			t.Fatalf("expected at least 1 unowned row, got %d (err=%v)", got, err)
		}
		n, err := AssignLegacyTenant(sysCtx, client, tenantB)
		if err != nil {
			t.Fatalf("assign legacy tenant: %v", err)
		}
		if n < 1 {
			t.Fatalf("expected >=1 收敛行, got %d", n)
		}
		reloaded := client.Chat.GetX(sysCtx, legacy.ID)
		if reloaded.TenantID != tenantB {
			t.Fatalf("legacy row tenant = %d, want %d", reloaded.TenantID, tenantB)
		}
		// 已归属的数据不能被覆盖
		reloadedA := client.Chat.GetX(sysCtx, chatA.ID)
		if reloadedA.TenantID != tenantA {
			t.Fatalf("已归属数据被误改：%d -> %d", tenantA, reloadedA.TenantID)
		}
		if got, err := CountUnownedChats(sysCtx, client); err != nil || got != 0 {
			t.Fatalf("收敛后仍有 %d 条未归属 (err=%v)", got, err)
		}
	})
}
