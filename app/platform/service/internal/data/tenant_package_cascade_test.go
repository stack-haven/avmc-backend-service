package data

import (
	"context"
	"io"
	"testing"

	pb "backend-service/api/platform/service/v1"
	"backend-service/app/platform/service/internal/data/ent/gen/role"
	"backend-service/pkg/auth/authz"
	"backend-service/pkg/entgo/viewer"

	"github.com/go-kratos/kratos/v2/log"
)

// TestPublishVersionCascadeTrimsTenantRoleMenus 验证套餐版本发布后的租户角色
// 菜单收敛真的落到了数据上。
//
// 回归目标（P0-2）：PublishVersion 收尾调用 cascadeTenantRoleMenus 时复用了请求
// ctx。该函数需要遍历所有绑定租户并改写其角色菜单，但 Role 受租户隐私规则约束，
// 平台操作员（无租户）调用时 Role 查询被 Deny，"带租户"操作员调用时只能命中自己
// 的租户，其余静默 continue —— 套餐裁掉菜单后租户角色仍保留旧权限（越权），而
// 发布接口返回成功。
func TestPublishVersionCascadeTrimsTenantRoleMenus(t *testing.T) {
	client := newTestClient(t)
	sysCtx := viewer.NewSystemContext(context.Background())

	authorizer, err := authz.NewAuthorizer("casbin", context.Background(),
		authz.WithAdapterType(authz.AdapterMemory))
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	menuKeep := client.Menu.Create().SetName("menu_keep").SetTitle("Keep").SetStatus(1).SaveX(sysCtx)
	menuDrop := client.Menu.Create().SetName("menu_drop").SetTitle("Drop").SetStatus(1).SaveX(sysCtx)

	tenantRow := client.Tenant.Create().
		SetName("被收敛租户").SetCode("cascade-tenant").SetLifecycleStatus(2).
		SaveX(sysCtx)
	group := client.TenantMenuPermissionGroup.Create().
		SetName("收敛套餐").SetCode("cascade-pkg").SetStatus(1).
		AddTenantIDs(tenantRow.ID).
		SaveX(sysCtx)
	v1 := client.TenantMenuPermissionGroupVersion.Create().
		SetGroupID(group.ID).SetVersion(1).SetState(1).
		AddMenuIDs(menuKeep.ID, menuDrop.ID).
		SaveX(sysCtx)
	client.TenantMenuPermissionGroup.UpdateOneID(group.ID).SetCurrentVersionID(v1.ID).SaveX(sysCtx)

	roleRow := client.Role.Create().
		SetTenantID(tenantRow.ID).SetName("租户管理员").SetStatus(1).
		AddMenuIDs(menuKeep.ID, menuDrop.ID).
		SaveX(sysCtx)

	d := &Data{db: client, authorizer: authorizer}
	repo := NewTenantMenuPermissionGroupRepo(d, log.NewStdLogger(io.Discard))

	// 平台操作员：有认证但没有租户归属。
	operatorCtx := context.Background()
	if _, err := repo.PublishVersion(operatorCtx, group.ID,
		&pb.TenantMenuPermissionGroupVersion{MenuIds: []uint32{menuKeep.ID}},
		"裁掉 menu_drop", 1, ""); err != nil {
		t.Fatalf("publish version: %v", err)
	}

	menus, err := client.Role.Query().
		Where(role.IDEQ(roleRow.ID)).
		QueryMenus().All(sysCtx)
	if err != nil {
		t.Fatalf("query role menus: %v", err)
	}
	if len(menus) != 1 || menus[0].ID != menuKeep.ID {
		ids := make([]uint32, 0, len(menus))
		for _, m := range menus {
			ids = append(ids, m.ID)
		}
		t.Fatalf("租户角色菜单未被收敛：期望 [%d]，实际 %v（跨租户级联被静默跳过）", menuKeep.ID, ids)
	}
}
