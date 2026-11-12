package data

import (
	"context"
	"fmt"

	"backend-service/app/ai/service/internal/data/ent/gen"
	"backend-service/app/ai/service/internal/data/ent/gen/chat"
	"backend-service/app/ai/service/internal/data/ent/gen/chatmessage"
	"backend-service/pkg/entgo/viewer"
)

// 本文件处理「ai 服务引入租户维度之前就存在的数据」。
//
// 历史背景：ai 此前没有 tenant_id，对话数据是全局共享的。补齐租户维度后，
// 存量行的 tenant_id 为 0，会被隐私规则永久过滤——数据既没丢，也没人能看见。
// 这是刻意的 fail-closed 选择：宁可暂时不可见，也不能默认泄露给某个租户。
//
// 因此这里提供两个显式工具：
//   - CountUnownedChats：让运维看到"有多少行没有归属"，而不是静默不可见；
//   - AssignLegacyTenant：把 tenant_id=0 的行归属到指定租户，归属目标必须由
//     人明确指定，代码不做任何猜测。
//
// 两者都可重复执行：AssignLegacyTenant 只改 tenant_id=0 的行，已归属的数据
// 不会被覆盖。

// CountUnownedChats 返回尚未归属任何租户的对话与消息数量（tenant_id = 0）。
func CountUnownedChats(ctx context.Context, client *gen.Client) (int, error) {
	ctx = viewer.NewSystemContext(ctx)
	chats, err := client.Chat.Query().Where(chat.TenantIDEQ(0)).Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("counting unowned chats: %w", err)
	}
	messages, err := client.ChatMessage.Query().Where(chatmessage.TenantIDEQ(0)).Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("counting unowned chat messages: %w", err)
	}
	return chats + messages, nil
}

// AssignLegacyTenant 把 tenant_id = 0 的历史对话与消息归属到 tenantID。
// 返回被修正的行数；tenantID 必须大于 0（0 代表"未归属"，不能作为目标）。
func AssignLegacyTenant(ctx context.Context, client *gen.Client, tenantID uint32) (int, error) {
	if tenantID == 0 {
		return 0, fmt.Errorf("legacy tenant id must be greater than 0")
	}
	ctx = viewer.NewSystemContext(ctx)
	chats, err := client.Chat.Update().
		Where(chat.TenantIDEQ(0)).
		SetTenantID(tenantID).
		Save(ctx)
	if err != nil {
		return 0, fmt.Errorf("assigning legacy tenant to chats: %w", err)
	}
	messages, err := client.ChatMessage.Update().
		Where(chatmessage.TenantIDEQ(0)).
		SetTenantID(tenantID).
		Save(ctx)
	if err != nil {
		return 0, fmt.Errorf("assigning legacy tenant to chat messages: %w", err)
	}
	return chats + messages, nil
}
