package data

import (
	"io"
	"testing"

	pbEnum "backend-service/api/common/enum"
	pbCore "backend-service/api/core/service/v1"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

func TestOperationLogRepoAppliesRoleDataScope(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()

	ctx := tenantContext(1)
	selfScope := int32(2)
	allScope := int32(1)
	selfRole := client.Role.Create().
		SetTenantID(1).
		SetName("operation-log-self-scope").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		SetDataScope(selfScope).
		SaveX(ctx)
	allRole := client.Role.Create().
		SetTenantID(1).
		SetName("operation-log-all-scope").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		SetDataScope(allScope).
		SaveX(ctx)
	actor := client.User.Create().
		SetTenantID(1).
		SetName("operation-log-actor").
		SetPassword("hashed-password").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		AddRoleIDs(selfRole.ID).
		SaveX(ctx)
	admin := client.User.Create().
		SetTenantID(1).
		SetName("operation-log-admin").
		SetPassword("hashed-password").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		AddRoleIDs(allRole.ID).
		SaveX(ctx)
	other := client.User.Create().
		SetTenantID(1).
		SetName("operation-log-other").
		SetPassword("hashed-password").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		SaveX(ctx)

	repo := NewOperationLogRepo(&Data{db: client}, log.NewStdLogger(io.Discard))
	if err := repo.Append(ctx, &pbCore.OperationLog{OperatorId: &actor.ID, Module: "user", Action: "create"}); err != nil {
		t.Fatalf("append actor log: %v", err)
	}
	if err := repo.Append(ctx, &pbCore.OperationLog{OperatorId: &other.ID, Module: "user", Action: "delete"}); err != nil {
		t.Fatalf("append other log: %v", err)
	}
	if err := repo.Append(ctx, &pbCore.OperationLog{Module: "system", Action: "sync"}); err != nil {
		t.Fatalf("append system log: %v", err)
	}

	actorCtx := tenantUserContext(1, actor.ID)
	items, total, err := repo.List(actorCtx, &pbCore.ListOperationLogsRequest{})
	if err != nil {
		t.Fatalf("List(actor) error = %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].GetOperatorId() != actor.ID {
		t.Fatalf("actor visible operation logs total=%d items=%+v", total, items)
	}
	if _, err = repo.Get(actorCtx, uint64(items[0].GetId())); err != nil {
		t.Fatalf("Get(actor own log) error = %v", err)
	}

	adminCtx := tenantUserContext(1, admin.ID)
	items, total, err = repo.List(adminCtx, &pbCore.ListOperationLogsRequest{})
	if err != nil {
		t.Fatalf("List(admin) error = %v", err)
	}
	if total != 3 || len(items) != 3 {
		t.Fatalf("admin visible operation logs total=%d items=%+v", total, items)
	}
	var otherLogID uint64
	for _, item := range items {
		if item.GetOperatorId() == other.ID {
			otherLogID = item.GetId()
			break
		}
	}
	if otherLogID == 0 {
		t.Fatalf("admin operation logs missing other operator item: %+v", items)
	}
	if _, err = repo.Get(actorCtx, otherLogID); !errors.IsNotFound(err) {
		t.Fatalf("Get(actor out of scope) error = %v, want not found", err)
	}
}
