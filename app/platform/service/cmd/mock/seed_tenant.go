//go:build mock

package main

import (
	"context"

	"backend-service/app/platform/service/internal/data/ent/gen"
	"backend-service/pkg/utils/crypto"
)

// mockUsers 汇总 seed 出来的用户引用，供后续角色/部门/项目 seed 复用。
type mockUsers struct {
	Admin    *gen.User // 技术中台管理租户 - 超级管理员
	Vben     *gen.User // 技术中台管理租户 - 超级管理员
	Jack     *gen.User // 技术中台管理租户 - 超级管理员
	Operator *gen.User // 技术中台管理租户 - 普通用户
	Tenant2  *gen.User // 客户企业租户 - 租户管理员
}

// tenantSpec 定义租户 seed 规格，保证 ID 固定、幂等可重复运行。
type tenantSpec struct {
	ID         uint32
	Name       string
	Code       string
	IsPlatform bool
}

// mockTenants 租户清单：ID 固定，后续迭代可在此追加新租户。
var mockTenants = []tenantSpec{
	{ID: 1, Name: "技术中台管理", Code: "tech-platform", IsPlatform: true},
	{ID: 2, Name: "客户企业", Code: "customer-a", IsPlatform: false},
}

// seedTenants 幂等维护租户与用户。
func seedTenants(ctx context.Context, c *gen.Client) (*mockUsers, error) {
	hash, err := crypto.HashPassword(mockPassword)
	if err != nil {
		return nil, err
	}

	platform := ensureTenant(ctx, c, 1, "技术中台管理", "tech-platform", true)
	customer := ensureTenant(ctx, c, 2, "客户企业", "customer-a", false)

	users := &mockUsers{
		Admin:    ensureUser(ctx, c, platform.ID, "admin", hash, "admin@example.com"),
		Vben:     ensureUser(ctx, c, platform.ID, "vben", hash, "vben@example.com"),
		Jack:     ensureUser(ctx, c, platform.ID, "jack", hash, "jack@example.com"),
		Operator: ensureUser(ctx, c, platform.ID, "operator", hash, "operator@example.com"),
		Tenant2:  ensureUser(ctx, c, customer.ID, "tenant2", hash, "tenant2@example.com"),
	}
	return users, nil
}
