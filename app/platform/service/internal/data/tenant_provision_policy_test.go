package data

import (
	"context"
	"io"
	"strconv"
	"testing"

	pb "backend-service/api/platform/service/v1"
	"backend-service/app/platform/service/internal/biz"
	"backend-service/app/platform/service/internal/data/ent/viewer"
	"backend-service/pkg/auth/authz"

	"github.com/go-kratos/kratos/v2/log"
)

// TestTenantProvisionSyncsCasbinPolicies 验证开通租户后管理员角色真的拿到了
// Casbin 策略。
//
// 回归目标（P0-1）：Provision 内部所有 Ent 操作都用 systemCtx，但历史上收尾的
// SyncRolePolicies/SyncUserRoles 复用了请求 ctx。SyncRolePolicies 会按操作者
// 租户过滤 Role 查询，于是「平台操作员开通租户」这一合法场景下目标角色不可见，
// 函数静默 return nil，新租户管理员最终零权限，且错误被丢弃无人发现。
//
// 因此本用例刻意用「无租户上下文」调用（平台控制面操作员的真实形态），并断言
// 策略确实落库。
func TestTenantProvisionSyncsCasbinPolicies(t *testing.T) {
	client := newTestClient(t)
	sysCtx := viewer.NewSystemContext(context.Background())

	authorizer, err := authz.NewAuthorizer("casbin", context.Background(),
		authz.WithAdapterType(authz.AdapterMemory))
	if err != nil {
		t.Fatalf("new authorizer: %v", err)
	}

	// 一个带后端权限标识的菜单，以及覆盖它的业务套餐。
	const authCode = "system:user:list"
	menuRow := client.Menu.Create().
		SetName("用户列表").
		SetTitle("User List").
		SetAuthCode(authCode).
		SetStatus(1).
		SaveX(sysCtx)
	group := client.TenantMenuPermissionGroup.Create().
		SetName("隔离套餐").
		SetCode("provision-policy").
		SetStatus(1).
		SaveX(sysCtx)
	version := client.TenantMenuPermissionGroupVersion.Create().
		SetGroupID(group.ID).
		SetVersion(1).
		SetState(1).
		AddMenuIDs(menuRow.ID).
		SaveX(sysCtx)
	client.TenantMenuPermissionGroup.UpdateOneID(group.ID).SetCurrentVersionID(version.ID).SaveX(sysCtx)

	d := &Data{db: client, authorizer: authorizer}
	repo := NewTenantRepo(d, log.NewStdLogger(io.Discard))

	// 平台操作员：有认证但没有租户归属 → 不能用请求 ctx 去查目标租户的数据。
	operatorCtx := context.Background()
	result, err := repo.Provision(operatorCtx, &biz.TenantProvisioning{
		Tenant:            &pb.Tenant{Name: ptr("隔离验证租户"), Code: ptr("provision-policy"), GroupIds: []uint32{group.ID}},
		AdminUsername:     "admin_probe",
		AdminPasswordHash: "hash-probe-123",
	})
	if err != nil {
		t.Fatalf("provision: %v", err)
	}

	allowed, err := authorizer.Enforce(sysCtx,
		authz.Subject(strconv.FormatUint(uint64(result.AdminRoleID), 10)),
		authz.Object(authCode),
		authz.Action("GET"),
		authz.Tenant(strconv.FormatUint(uint64(result.Tenant.GetId()), 10)),
	)
	if err != nil {
		t.Fatalf("enforce: %v", err)
	}
	if !allowed {
		t.Fatalf("新租户管理员角色 %d 没有任何 Casbin 策略：权限同步被静默跳过", result.AdminRoleID)
	}
}
