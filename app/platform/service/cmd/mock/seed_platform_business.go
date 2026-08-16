//go:build mock

package main

import (
	"context"

	"backend-service/app/platform/service/internal/data"
	"backend-service/app/platform/service/internal/data/ent/gen"
	"backend-service/app/platform/service/internal/data/ent/gen/menu"
	authzEngine "backend-service/pkg/auth/authz"
)

// seedPackages 幂等维护套餐（租户菜单权限组）。
func seedPackages(ctx context.Context, c *gen.Client) error {
	allIDs, _ := c.Menu.Query().IDs(ctx)
	ensurePkg(ctx, c, "全功能管理员套餐", "full-admin", true, allIDs)

	// 基础业务套餐：仅包含核心目录及其子菜单。
	basicPaths := []string{"/tenant", "/org", "/perm", "/file", "/notif", "/dashboard"}
	basicDirs, _ := c.Menu.Query().Where(menu.PathIn(basicPaths...)).IDs(ctx)
	basicSet := make(map[uint32]bool)
	for _, id := range basicDirs {
		basicSet[id] = true
	}
	allMenus, _ := c.Menu.Query().All(ctx)
	basicIDs := make([]uint32, 0)
	for _, m := range allMenus {
		if basicSet[*m.ParentID] || basicSet[m.ID] {
			basicIDs = append(basicIDs, m.ID)
		}
	}
	basicIDs = append(basicIDs, basicDirs...)
	ensurePkg(ctx, c, "基础业务套餐", "basic-business", true, basicIDs)
	return nil
}

// seedRoles 幂等维护角色 + 用户角色绑定 + Casbin 策略同步。
func seedRoles(ctx context.Context, c *gen.Client, authorizer authzEngine.Authorizer, users *mockUsers) error {
	allIDs, _ := c.Menu.Query().IDs(ctx)

	basicPaths := []string{"/tenant", "/org", "/perm", "/file", "/notif", "/dashboard"}
	basicDirs, _ := c.Menu.Query().Where(menu.PathIn(basicPaths...)).IDs(ctx)
	basicSet := make(map[uint32]bool)
	for _, id := range basicDirs {
		basicSet[id] = true
	}
	allMenus, _ := c.Menu.Query().All(ctx)
	basicIDs := make([]uint32, 0)
	for _, m := range allMenus {
		if basicSet[*m.ParentID] || basicSet[m.ID] {
			basicIDs = append(basicIDs, m.ID)
		}
	}
	basicIDs = append(basicIDs, basicDirs...)

	// 技术中台管理租户（ID=1）：超级管理员（全菜单）+ 普通用户（基础菜单）
	sa := ensureRole(ctx, c, 1, "超级管理员", allIDs, true)
	normalRole := ensureRole(ctx, c, 1, "普通用户", basicIDs, false)

	// 客户企业租户（ID=2）：租户管理员（基础菜单）
	t2Role := ensureRole(ctx, c, 2, "租户管理员", basicIDs, true)

	// 用户-角色绑定（数据库：system_user_roles）
	c.User.UpdateOneID(users.Admin.ID).AddRoleIDs(sa.ID).Exec(ctx)
	c.User.UpdateOneID(users.Vben.ID).AddRoleIDs(sa.ID).Exec(ctx)
	c.User.UpdateOneID(users.Jack.ID).AddRoleIDs(sa.ID).Exec(ctx)
	c.User.UpdateOneID(users.Tenant2.ID).AddRoleIDs(t2Role.ID).Exec(ctx)

	// Casbin p 规则（角色→权限，多次运行幂等：先清旧再重建）
	if err := data.SyncRolePolicies(ctx, c, authorizer, 1, sa.ID); err != nil {
		return err
	}
	if err := data.SyncRolePolicies(ctx, c, authorizer, 1, normalRole.ID); err != nil {
		return err
	}
	if err := data.SyncRolePolicies(ctx, c, authorizer, 2, t2Role.ID); err != nil {
		return err
	}

	// Casbin g 规则（用户→角色，否则 g(r.sub, p.sub) 不匹配，业务接口全部 403）
	if err := data.SyncUserRoles(ctx, c, authorizer, 1, users.Admin.ID, []uint32{sa.ID}); err != nil {
		return err
	}
	if err := data.SyncUserRoles(ctx, c, authorizer, 1, users.Vben.ID, []uint32{sa.ID}); err != nil {
		return err
	}
	if err := data.SyncUserRoles(ctx, c, authorizer, 1, users.Jack.ID, []uint32{sa.ID}); err != nil {
		return err
	}
	if err := data.SyncUserRoles(ctx, c, authorizer, 2, users.Tenant2.ID, []uint32{t2Role.ID}); err != nil {
		return err
	}
	return nil
}

// seedOrgs 幂等维护部门、岗位、项目。
func seedOrgs(ctx context.Context, c *gen.Client, users *mockUsers) error {
	// 部门
	t1d, err := ensureDept(ctx, c, 1, "总公司", 0, users.Admin.ID)
	if err != nil {
		return err
	}
	t1tech, err := ensureDept(ctx, c, 1, "技术部", t1d.ID, users.Vben.ID)
	if err != nil {
		return err
	}
	t2d, err := ensureDept(ctx, c, 2, "客户企业", 0, users.Tenant2.ID)
	if err != nil {
		return err
	}
	c.User.UpdateOneID(users.Admin.ID).SetDeptID(t1d.ID).Exec(ctx)
	c.User.UpdateOneID(users.Jack.ID).SetDeptID(t1d.ID).Exec(ctx)
	c.User.UpdateOneID(users.Vben.ID).SetDeptID(t1tech.ID).Exec(ctx)
	c.User.UpdateOneID(users.Operator.ID).SetDeptID(t1tech.ID).Exec(ctx)
	c.User.UpdateOneID(users.Tenant2.ID).SetDeptID(t2d.ID).Exec(ctx)

	// 岗位
	for _, nm := range []string{"技术总监", "运营经理", "开发工程师"} {
		ensurePost(ctx, c, 1, nm)
	}
	ensurePost(ctx, c, 2, "管理员")

	// 项目
	ensureProject(ctx, c, 1, "GEO内容工程", "geo-engine", users.Admin.ID, []uint32{users.Admin.ID, users.Vben.ID})
	ensureProject(ctx, c, 2, "客户项目A", "customer-a", users.Tenant2.ID, []uint32{users.Tenant2.ID})
	return nil
}
