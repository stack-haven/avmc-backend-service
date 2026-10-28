//go:build mock

package main

import (
	"context"
	"strings"

	"backend-service/app/platform/service/internal/data"
	"backend-service/app/platform/service/internal/data/ent/gen"
	"backend-service/app/platform/service/internal/data/ent/gen/menu"
	authzEngine "backend-service/pkg/auth/authz"
)

// basicMenuIDs 计算「基础业务」菜单集合：核心目录及其子菜单与页面级按钮，
// 排除敏感页（安全审计三页）与平台运营专属页（开发者目录）。
// 返回 (页面目录集, 按钮集)：按钮必须单独返回，由调用方按角色分级授予：
//   - 普通用户：页面 + 只读按钮（Get/List），否则列表接口也会 403；
//   - 租户管理员 / 基础套餐：页面 + 全部按钮（租户自治全能力）。
//
// 2026-10-22 修复：此前按钮从未被收录（按钮 parent 是页面而非目录），
// 导致租户管理员对全部写接口 403（Q1 根因）。
func basicMenuIDs(ctx context.Context, c *gen.Client) (pageIDs, btnIDs []uint32) {
	basicPaths := []string{"/tenant", "/org", "/perm", "/file", "/notif", "/dashboard"}
	basicDirs, _ := c.Menu.Query().Where(menu.PathIn(basicPaths...)).IDs(ctx)
	basicSet := make(map[uint32]bool)
	for _, id := range basicDirs {
		basicSet[id] = true
	}
	// 敏感/运营专属页排除集：安全审计三页（含其下按钮）+ 开发者目录。
	excludePaths := []string{"/perm/operation-log", "/perm/login-log", "/perm/session", "/developer"}
	excludeIDs, _ := c.Menu.Query().Where(menu.PathHasPrefix("/developer")).IDs(ctx)
	auditIDs, _ := c.Menu.Query().Where(menu.PathIn(excludePaths[:3]...)).IDs(ctx)
	excludeIDs = append(excludeIDs, auditIDs...)
	// 排除页下的按钮一并排除
	excludeBtnIDs, _ := c.Menu.Query().Where(menu.ParentIDIn(excludeIDs...)).IDs(ctx)
	excluded := make(map[uint32]bool)
	for _, id := range append(excludeIDs, excludeBtnIDs...) {
		excluded[id] = true
	}

	allMenus, _ := c.Menu.Query().All(ctx)
	pageIDs = make([]uint32, 0)
	included := make(map[uint32]bool)
	for _, m := range allMenus {
		if excluded[m.ID] || (m.Type != nil && *m.Type == 3) {
			continue
		}
		if basicSet[*m.ParentID] || basicSet[m.ID] {
			pageIDs = append(pageIDs, m.ID)
			included[m.ID] = true
		}
	}
	for _, id := range basicDirs {
		if !excluded[id] {
			pageIDs = append(pageIDs, id)
			included[id] = true
		}
	}
	// 个人中心：所有角色（含普通用户/租户管理员）必须可见。
	profileIDs, _ := c.Menu.Query().Where(menu.NameEQ("ProfileCenter")).IDs(ctx)
	pageIDs = append(pageIDs, profileIDs...)
	for _, id := range profileIDs {
		included[id] = true
	}
	// 页面级按钮：parent 属于收录集的按钮全部纳入（敏感页按钮已被 excluded 拦截）。
	btnIDs = make([]uint32, 0)
	for _, m := range allMenus {
		if m.Type != nil && *m.Type == 3 && !excluded[m.ID] && included[*m.ParentID] {
			btnIDs = append(btnIDs, m.ID)
		}
	}
	return pageIDs, btnIDs
}

// readOnlyButtonIDs 过滤只读按钮（operation 末段动词为 Get/List）。
func readOnlyButtonIDs(ctx context.Context, c *gen.Client, btnIDs []uint32) []uint32 {
	if len(btnIDs) == 0 {
		return nil
	}
	rows, _ := c.Menu.Query().Where(menu.IDIn(btnIDs...)).All(ctx)
	out := make([]uint32, 0, len(rows))
	for _, m := range rows {
		if m.AuthCode == nil {
			continue
		}
		seg := strings.Split(*m.AuthCode, "/")
		last := seg[len(seg)-1]
		if strings.HasPrefix(last, "Get") || strings.HasPrefix(last, "List") {
			out = append(out, m.ID)
		}
	}
	return out
}

// seedPackages 幂等维护套餐（租户菜单权限组）。
func seedPackages(ctx context.Context, c *gen.Client) error {
	allIDs, _ := c.Menu.Query().IDs(ctx)
	ensurePkg(ctx, c, "全功能管理员套餐", "full-admin", true, allIDs)

	// 基础业务套餐：核心目录、子菜单及全部页面按钮（套餐是租户可用上限），
	// 排除敏感页与开发者目录。
	basicPages, basicBtns := basicMenuIDs(ctx, c)
	ensurePkg(ctx, c, "基础业务套餐", "basic-business", true, append(basicPages, basicBtns...))
	return nil
}

// seedRoles 幂等维护角色 + 用户角色绑定 + Casbin 策略同步。
func seedRoles(ctx context.Context, c *gen.Client, authorizer authzEngine.Authorizer, users *mockUsers) error {
	allIDs, _ := c.Menu.Query().IDs(ctx)

	// 基础菜单集：复用 basicMenuIDs（已排除安全审计页与开发者目录）。
	// 按钮按角色分级：普通用户仅只读（Get/List），租户管理员全量（租户自治）。
	basicPages, basicBtns := basicMenuIDs(ctx, c)
	readOnlyBtns := readOnlyButtonIDs(ctx, c, basicBtns)

	// 技术中台管理租户（ID=1）：超级管理员（全菜单）+ 普通用户（基础菜单+只读按钮）
	sa := ensureRole(ctx, c, 1, "超级管理员", allIDs, true)
	normalRole := ensureRole(ctx, c, 1, "普通用户", append(basicPages, readOnlyBtns...), false)

	// 客户企业租户（ID=2）：租户管理员（基础菜单+全部按钮 + Evie 语音智能引擎菜单与按钮，供多租户隔离验收）
	eviePageIDs, _ := c.Menu.Query().Where(menu.PathHasPrefix("/evie")).IDs(ctx)
	evieBtnIDs, _ := c.Menu.Query().Where(menu.ParentIDIn(eviePageIDs...)).IDs(ctx)
	evieAllIDs := append(eviePageIDs, evieBtnIDs...)
	// 租户运行时参数（/parameters/current 系列）是租户自助能力，但按钮挂在平台
	// 「参数管理」页下（/sys 不在 basic 集）——单独授予策略（UI 入口待租户自助视图批次）。
	paramCurrentIDs, _ := c.Menu.Query().Where(menu.NameIn("ParamCurrentQuery", "ParamCurrentSet", "ParamCurrentReset")).IDs(ctx)
	t2Role := ensureRole(ctx, c, 2, "租户管理员", append(append(append(basicPages, basicBtns...), evieAllIDs...), paramCurrentIDs...), true)

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
