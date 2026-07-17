package data

import (
	"io"
	"testing"

	pbEnum "backend-service/api/common/enum"
	pbCore "backend-service/api/core/service/v1"

	"github.com/go-kratos/kratos/v2/log"
)

func TestLoginLogRepoTenantIsolationAndFiltering(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()
	repo := NewLoginLogRepo(&Data{db: client}, log.NewStdLogger(io.Discard))

	adminScope := int32(1)
	roleOne := client.Role.Create().
		SetName("tenant-one-login-admin").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		SetDataScope(adminScope).
		SaveX(tenantContext(1))
	adminOne := client.User.Create().
		SetName("tenant-one-login-user").
		SetPassword("hashed-password").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		AddRoleIDs(roleOne.ID).
		SaveX(tenantContext(1))
	roleTwo := client.Role.Create().
		SetName("tenant-two-login-admin").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		SetDataScope(adminScope).
		SaveX(tenantContext(2))
	adminTwo := client.User.Create().
		SetName("tenant-two-login-user").
		SetPassword("hashed-password").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		AddRoleIDs(roleTwo.ID).
		SaveX(tenantContext(2))

	success := "success"
	failure := "failure"
	if err := repo.Append(tenantContext(1), &pbCore.LoginLog{
		TenantId: 1, UserId: &adminOne.ID, Identity: "admin", LoginType: "username", Result: success,
	}); err != nil {
		t.Fatalf("append tenant 1 success: %v", err)
	}
	if err := repo.Append(tenantContext(1), &pbCore.LoginLog{
		TenantId: 1, Identity: "unknown", LoginType: "username", Result: failure,
	}); err != nil {
		t.Fatalf("append tenant 1 failure: %v", err)
	}
	if err := repo.Append(tenantContext(2), &pbCore.LoginLog{
		TenantId: 2, UserId: &adminTwo.ID, Identity: "admin", LoginType: "username", Result: success,
	}); err != nil {
		t.Fatalf("append tenant 2 success: %v", err)
	}

	items, total, err := repo.List(tenantUserContext(1, adminOne.ID), &pbCore.ListLoginLogsRequest{Result: &success})
	if err != nil {
		t.Fatalf("list tenant 1: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].GetTenantId() != 1 {
		t.Fatalf("tenant 1 filtered logs = total:%d items:%+v", total, items)
	}
	items, total, err = repo.List(tenantUserContext(2, adminTwo.ID), &pbCore.ListLoginLogsRequest{})
	if err != nil {
		t.Fatalf("list tenant 2: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].GetTenantId() != 2 {
		t.Fatalf("tenant 2 logs = total:%d items:%+v", total, items)
	}
}

func TestLoginLogRepoAppliesRoleDataScope(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()
	repo := NewLoginLogRepo(&Data{db: client}, log.NewStdLogger(io.Discard))

	ctx := tenantContext(1)
	selfScope := int32(2)
	allScope := int32(1)
	selfRole := client.Role.Create().
		SetName("login-log-self-scope").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		SetDataScope(selfScope).
		SaveX(ctx)
	allRole := client.Role.Create().
		SetName("login-log-all-scope").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		SetDataScope(allScope).
		SaveX(ctx)
	actor := client.User.Create().
		SetName("login-log-actor").
		SetPassword("hashed-password").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		AddRoleIDs(selfRole.ID).
		SaveX(ctx)
	admin := client.User.Create().
		SetName("login-log-admin").
		SetPassword("hashed-password").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		AddRoleIDs(allRole.ID).
		SaveX(ctx)
	other := client.User.Create().
		SetName("login-log-other").
		SetPassword("hashed-password").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		SaveX(ctx)

	success := "success"
	if err := repo.Append(ctx, &pbCore.LoginLog{UserId: &actor.ID, Identity: "actor", LoginType: "username", Result: success}); err != nil {
		t.Fatalf("append actor log: %v", err)
	}
	if err := repo.Append(ctx, &pbCore.LoginLog{UserId: &other.ID, Identity: "other", LoginType: "username", Result: success}); err != nil {
		t.Fatalf("append other log: %v", err)
	}
	if err := repo.Append(ctx, &pbCore.LoginLog{Identity: "anonymous", LoginType: "username", Result: success}); err != nil {
		t.Fatalf("append anonymous log: %v", err)
	}

	items, total, err := repo.List(tenantUserContext(1, actor.ID), &pbCore.ListLoginLogsRequest{})
	if err != nil {
		t.Fatalf("List(actor) error = %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].GetUserId() != actor.ID {
		t.Fatalf("actor visible login logs total=%d items=%+v", total, items)
	}
	if _, err = repo.Get(tenantUserContext(1, actor.ID), items[0].GetId()); err != nil {
		t.Fatalf("Get(actor own log) error = %v", err)
	}

	items, total, err = repo.List(tenantUserContext(1, admin.ID), &pbCore.ListLoginLogsRequest{})
	if err != nil {
		t.Fatalf("List(admin) error = %v", err)
	}
	if total != 3 || len(items) != 3 {
		t.Fatalf("admin visible login logs total=%d items=%+v", total, items)
	}
}
