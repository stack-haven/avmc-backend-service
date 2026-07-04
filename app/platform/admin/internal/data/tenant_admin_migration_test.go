package data

import (
	"io"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
)

func TestBackfillTenantAdminRoleMarker(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()
	ctx := tenantContext(1)
	admin := client.Role.Create().SetName("租户管理员").SetStatus(1).SaveX(ctx)
	regular := client.Role.Create().SetName("普通角色").SetStatus(1).SaveX(ctx)

	if err := backfillTenantAdminRoleMarker(
		systemContext(),
		client,
		log.NewHelper(log.NewStdLogger(io.Discard)),
	); err != nil {
		t.Fatalf("backfill tenant admin marker: %v", err)
	}
	if !client.Role.GetX(systemContext(), admin.ID).IsTenantAdmin {
		t.Fatal("tenant admin role was not marked")
	}
	if client.Role.GetX(systemContext(), regular.ID).IsTenantAdmin {
		t.Fatal("regular role was incorrectly marked")
	}
}
