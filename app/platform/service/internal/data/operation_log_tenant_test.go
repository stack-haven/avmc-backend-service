package data

import (
	"context"
	"io"
	"testing"

	pb "backend-service/api/platform/service/v1"
	"backend-service/app/platform/service/internal/data/ent/gen"
	"backend-service/app/platform/service/internal/data/ent/gen/operationlog"
	"backend-service/app/platform/service/internal/data/ent/viewer"

	"github.com/go-kratos/kratos/v2/log"
)

// TestOperationLogAppendRejectsCrossTenantWrites 验证审计委托不能跨租户写入。
//
// 回归目标（P0-3）：CreateOperationLog 是跨服务审计委托入口（平台侧免 Casbin
// 鉴权，但保留认证），调用方会原样转发调用者的 Authorization。历史实现无条件
// 用请求体里的 tenant_id 覆盖 Ent viewer，于是任意已登录用户可以往任意租户写入
// 审计条目并伪造操作人——认证身份与写入租户之间没有任何一致性校验。
func TestOperationLogAppendRejectsCrossTenantWrites(t *testing.T) {
	client := newTestClient(t)
	sysCtx := viewer.NewSystemContext(context.Background())

	const ownTenant, victimTenant = uint32(1), uint32(2)
	client.Tenant.Create().SetID(ownTenant).SetName("自有租户").SetCode("own").SaveX(sysCtx)
	client.Tenant.Create().SetID(victimTenant).SetName("受害租户").SetCode("victim").SaveX(sysCtx)

	repo := NewOperationLogRepo(&Data{db: client}, log.NewStdLogger(io.Discard))

	// 调用者已认证属于 ownTenant，却声称要为 victimTenant 写审计。
	callerCtx := tenantContext(ownTenant)
	err := repo.Append(callerCtx, &pb.OperationLog{
		TenantId:     victimTenant,
		OperatorName: ptr("forged-operator"),
		Module:       "system",
		Action:       "delete",
	})
	if err == nil {
		t.Fatal("expected cross-tenant audit write to be rejected")
	}

	written := client.OperationLog.Query().
		Where(operationlog.TenantIDEQ(victimTenant)).
		CountX(sysCtx)
	if written != 0 {
		t.Fatalf("victim tenant received %d forged audit rows", written)
	}
}

// TestOperationLogAppendAcceptsOwnTenant 保证正常路径不被收紧破坏：
// 调用者为自己所属租户写审计必须成功，且 tenant_id 落到认证租户上。
func TestOperationLogAppendAcceptsOwnTenant(t *testing.T) {
	client := newTestClient(t)
	sysCtx := viewer.NewSystemContext(context.Background())

	const ownTenant = uint32(1)
	client.Tenant.Create().SetID(ownTenant).SetName("自有租户").SetCode("own").SaveX(sysCtx)

	repo := NewOperationLogRepo(&Data{db: client}, log.NewStdLogger(io.Discard))
	if err := repo.Append(tenantContext(ownTenant), &pb.OperationLog{
		TenantId:     ownTenant,
		OperatorName: ptr("real-operator"),
		Module:       "system",
		Action:       "update",
	}); err != nil {
		t.Fatalf("append audit for own tenant: %v", err)
	}

	rows, err := client.OperationLog.Query().
		Where(operationlog.TenantIDEQ(ownTenant)).
		All(sysCtx)
	if err != nil {
		t.Fatalf("query audit rows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 audit row, got %d", len(rows))
	}
	if rows[0].TenantID != ownTenant {
		t.Fatalf("audit row tenant = %d, want %d", rows[0].TenantID, ownTenant)
	}
	var _ *gen.OperationLog = rows[0]
}
