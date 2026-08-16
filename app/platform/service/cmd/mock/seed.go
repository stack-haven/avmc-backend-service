//go:build mock

package main

import (
	"context"
	"fmt"

	"backend-service/app/platform/service/internal/data/ent/gen"
	authzEngine "backend-service/pkg/auth/authz"
)

// seed 按依赖顺序编排各 seed 函数，全程幂等，可多次运行增量维护。
//
// 顺序依赖：
//  1. 租户（后续用户/角色/部门都依赖 tenant_id）
//  2. 菜单（后续按钮/套餐/角色都依赖菜单 ID）
//  3. 按钮（依赖菜单 name→ID 映射）
//  4. 套餐（依赖全量菜单 ID）
//  5. 角色（依赖菜单 ID + 用户 + Casbin 策略同步）
//  6. 组织（部门/岗位/项目，依赖用户 ID）
func seed(ctx context.Context, client *gen.Client, authorizer authzEngine.Authorizer) error {
	users, err := seedTenants(ctx, client)
	if err != nil {
		return fmt.Errorf("seed tenants: %w", err)
	}

	if err := seedConfig(ctx, client); err != nil {
		return fmt.Errorf("seed config: %w", err)
	}

	platformMenuMap, err := seedMenus(ctx, client)
	if err != nil {
		return fmt.Errorf("seed menus: %w", err)
	}

	// Evie 产品服务菜单：seed 后 allIDs 会包含 evie 菜单，自动进入全功能套餐与超级管理员角色。
	if _, err := seedEvie(ctx, client); err != nil {
		return fmt.Errorf("seed evie menus: %w", err)
	}

	if err := seedButtons(ctx, client, platformMenuMap); err != nil {
		return fmt.Errorf("seed buttons: %w", err)
	}

	if err := seedPackages(ctx, client); err != nil {
		return fmt.Errorf("seed packages: %w", err)
	}

	if err := seedRoles(ctx, client, authorizer, users); err != nil {
		return fmt.Errorf("seed roles: %w", err)
	}

	if err := seedOrgs(ctx, client, users); err != nil {
		return fmt.Errorf("seed orgs: %w", err)
	}

	return nil
}
