//go:build mock

package main

import (
	"context"
	"time"

	"backend-service/app/platform/service/internal/data/ent/gen"
	"backend-service/app/platform/service/internal/data/ent/gen/dept"
	"backend-service/app/platform/service/internal/data/ent/gen/menu"
	"backend-service/app/platform/service/internal/data/ent/gen/post"
	"backend-service/app/platform/service/internal/data/ent/gen/project"
	"backend-service/app/platform/service/internal/data/ent/gen/role"
	"backend-service/app/platform/service/internal/data/ent/gen/tenant"
	"backend-service/app/platform/service/internal/data/ent/gen/tenantmenupermissiongroup"
	"backend-service/app/platform/service/internal/data/ent/gen/user"
)

// ── 菜单 ─────────────────────────────────────────────────

func menuDir(ctx context.Context, c *gen.Client, s mockMenuSpec, pid uint32) *gen.Menu {
	return menuX(ctx, c, s, pid, 1)
}

func menuItem(ctx context.Context, c *gen.Client, s mockMenuSpec, pid uint32) *gen.Menu {
	return menuX(ctx, c, s, pid, 2)
}

// menuX 幂等 upsert 目录/页面菜单：按 name 查找，存在则更新，不存在则创建。
func menuX(ctx context.Context, c *gen.Client, s mockMenuSpec, pid uint32, typ int32) *gen.Menu {
	ex, err := c.Menu.Query().Where(menu.NameEQ(s.Name)).Only(ctx)
	if err != nil {
		return c.Menu.Create().SetName(s.Name).SetTitle(s.Title).SetPath(s.Path).
			SetComponent(s.Component).SetIcon(s.Icon).SetType(typ).SetSort(s.Sort).
			SetNillableAuthCode(&s.AuthCode).SetNillableParentID(&pid).SaveX(ctx)
	}
	c.Menu.UpdateOneID(ex.ID).SetTitle(s.Title).SetPath(s.Path).
		SetComponent(s.Component).SetIcon(s.Icon).SetType(typ).SetSort(s.Sort).
		SetNillableAuthCode(&s.AuthCode).SetNillableParentID(&pid).Exec(ctx)
	return ex
}

// menuBtn 幂等 upsert 权限按钮：按 name 查找，存在则更新，不存在则创建。
func menuBtn(ctx context.Context, c *gen.Client, s mockMenuSpec, pid uint32) {
	ex, err := c.Menu.Query().Where(menu.NameEQ(s.Name)).Only(ctx)
	if err != nil {
		c.Menu.Create().SetName(s.Name).SetTitle(s.Title).SetPath(s.Path).
			SetComponent(s.Component).SetIcon(s.Icon).SetType(3).SetSort(s.Sort).
			SetNillableAuthCode(&s.AuthCode).SetNillableParentID(&pid).SaveX(ctx)
		return
	}
	c.Menu.UpdateOneID(ex.ID).
		SetTitle(s.Title).
		SetSort(s.Sort).
		SetNillableAuthCode(&s.AuthCode).
		SetNillableParentID(&pid).
		Exec(ctx)
}

// ── 租户 / 用户 ─────────────────────────────────────────

// ensureTenant 幂等维护租户：按 code 查找，存在则更新关键字段，不存在则创建。
// 显式 SetID 保证固定 ID（如平台租户=1），多次运行不会漂移。
func ensureTenant(ctx context.Context, c *gen.Client, id uint32, name, code string, isPlatform bool) *gen.Tenant {
	t, err := c.Tenant.Query().Where(tenant.CodeEQ(code)).Only(ctx)
	if err == nil {
		c.Tenant.UpdateOneID(t.ID).SetName(name).SetIsPlatform(isPlatform).
			SetLifecycleStatus(2).Exec(ctx)
		return t
	}
	return c.Tenant.Create().
		SetID(id).
		SetName(name).
		SetCode(code).
		SetIsPlatform(isPlatform).
		SetLifecycleStatus(2).
		SetActivatedAt(time.Now()).
		SaveX(ctx)
}

// ensureUser 幂等维护用户：按 tenant+name 查找，不存在则创建。
func ensureUser(ctx context.Context, c *gen.Client, tid uint32, name, hash, email string) *gen.User {
	u, err := c.User.Query().Where(user.TenantIDEQ(tid), user.NameEQ(name)).Only(ctx)
	if err != nil {
		u = c.User.Create().SetTenantID(tid).SetName(name).SetPassword(hash).SetEmail(email).SetStatus(1).SaveX(ctx)
	}
	return u
}

// ── 套餐 / 角色 ─────────────────────────────────────────

// ensurePkg 幂等维护套餐（租户菜单权限组）：按 code 查找，增量补菜单，不存在则创建并发布 v1 版本。
func ensurePkg(ctx context.Context, c *gen.Client, name, code string, isSystem bool, menuIDs []uint32) *gen.TenantMenuPermissionGroup {
	ex, err := c.TenantMenuPermissionGroup.Query().Where(tenantmenupermissiongroup.CodeEQ(code)).Only(ctx)
	if err != nil {
		pkg := c.TenantMenuPermissionGroup.Create().SetName(name).SetCode(code).SetNillableIsSystem(&isSystem).AddMenuIDs(menuIDs...).SaveX(ctx)
		ver := c.TenantMenuPermissionGroupVersion.Create().SetGroupID(pkg.ID).SetVersion(1).SetState(1).AddMenuIDs(menuIDs...).SaveX(ctx)
		c.TenantMenuPermissionGroup.UpdateOneID(pkg.ID).SetCurrentVersionID(ver.ID).Exec(ctx)
		return pkg
	}
	existingIDs, _ := ex.QueryMenus().IDs(ctx)
	if missing := missingIDs(existingIDs, menuIDs); len(missing) > 0 {
		c.TenantMenuPermissionGroup.UpdateOneID(ex.ID).AddMenuIDs(missing...).Exec(ctx)
	}
	return ex
}

// ensureRole 幂等维护角色：按 tenant+name 查找，增量补菜单，不存在则创建。
func ensureRole(ctx context.Context, c *gen.Client, tid uint32, name string, menuIDs []uint32, isTA bool) *gen.Role {
	r, err := c.Role.Query().Where(role.TenantIDEQ(tid), role.NameEQ(name)).Only(ctx)
	if err != nil {
		r = c.Role.Create().SetTenantID(tid).SetName(name).SetDataScope(1).SetNillableIsTenantAdmin(&isTA).SetStatus(1).AddMenuIDs(menuIDs...).SaveX(ctx)
	} else {
		existingIDs, _ := r.QueryMenus().IDs(ctx)
		if missing := missingIDs(existingIDs, menuIDs); len(missing) > 0 {
			c.Role.UpdateOneID(r.ID).AddMenuIDs(missing...).Exec(ctx)
		}
	}
	return r
}

// ── 部门 / 岗位 / 项目 ──────────────────────────────────

func ensureDept(ctx context.Context, c *gen.Client, tid uint32, name string, pid, lid uint32) (*gen.Dept, error) {
	ex, err := c.Dept.Query().Where(dept.TenantIDEQ(tid), dept.NameEQ(name)).Only(ctx)
	if err != nil {
		return c.Dept.Create().SetTenantID(tid).SetName(name).SetParentID(pid).SetNillableLeaderID(&lid).SetStatus(1).Save(ctx)
	}
	return ex, nil
}

func ensurePost(ctx context.Context, c *gen.Client, tid uint32, name string) {
	_, err := c.Post.Query().Where(post.TenantIDEQ(tid), post.NameEQ(name)).Only(ctx)
	if err != nil {
		c.Post.Create().SetTenantID(tid).SetName(name).SetStatus(1).SaveX(ctx)
	}
}

func ensureProject(ctx context.Context, c *gen.Client, tid uint32, name, code string, oid uint32, mids []uint32) {
	_, err := c.Project.Query().Where(project.TenantIDEQ(tid), project.CodeEQ(code)).Only(ctx)
	if err != nil {
		c.Project.Create().SetTenantID(tid).SetName(name).SetCode(code).SetOwnerID(oid).AddMemberIDs(mids...).SetStatus(1).SaveX(ctx)
	}
}

// ── 工具 ─────────────────────────────────────────────────

// missingIDs 返回 desired 中不在 existing 的 ID（用于增量维护）。
func missingIDs(existing, desired []uint32) []uint32 {
	seen := make(map[uint32]struct{}, len(existing))
	for _, id := range existing {
		seen[id] = struct{}{}
	}
	missing := make([]uint32, 0)
	for _, id := range desired {
		if _, ok := seen[id]; !ok {
			missing = append(missing, id)
		}
	}
	return missing
}

// ── 菜单/按钮 seed 通用逻辑 ──────────────────────────────

// seedMenuTree 通用：根据 menuSeed 列表（父在前）幂等 seed 菜单目录/页面，返回 name→ID 映射。
func seedMenuTree(ctx context.Context, c *gen.Client, seeds []menuSeed) (map[string]uint32, error) {
	menuMap := make(map[string]uint32, len(seeds))
	for _, s := range seeds {
		var parentID uint32
		if s.Parent != "" {
			parentID = menuMap[s.Parent]
		}
		spec := mockMenuSpec{Name: s.Name, Title: s.Title, Path: s.Path, Component: s.Component, Icon: s.Icon, Type: s.Type, Sort: s.Sort}
		var m *gen.Menu
		if s.Type == 1 {
			m = menuDir(ctx, c, spec, parentID)
		} else {
			m = menuItem(ctx, c, spec, parentID)
		}
		menuMap[s.Name] = m.ID
	}
	return menuMap, nil
}

// seedButtonList 通用：根据 buttonSpec 列表幂等 upsert 权限按钮。
func seedButtonList(ctx context.Context, c *gen.Client, buttons []buttonSpec, menuMap map[string]uint32) error {
	for _, b := range buttons {
		parentID, ok := menuMap[b.Parent]
		if !ok {
			// 父菜单不存在（可能被删）时跳过，避免悬空按钮。
			continue
		}
		menuBtn(ctx, c, mockMenuSpec{
			Name: b.Name, Title: b.Title, AuthCode: b.Operation, Type: 3, Sort: b.Sort,
		}, parentID)
	}
	return nil
}
