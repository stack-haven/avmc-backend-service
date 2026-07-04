package data

import (
	"context"
	"io"
	"sort"
	"testing"

	pbEnum "backend-service/api/common/enum"
	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/internal/data/ent/gen/role"
	"backend-service/app/platform/admin/internal/data/ent/gen/user"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

func TestUserRepoAppliesRoleDataScope(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()
	ctx := tenantContext(1)
	root := client.Dept.Create().SetName("root").SetAncestors([]int{}).SaveX(ctx)
	child := client.Dept.Create().
		SetName("child").
		SetParentID(root.ID).
		SetAncestors([]int{int(root.ID)}).
		SaveX(ctx)
	other := client.Dept.Create().SetName("other").SetAncestors([]int{}).SaveX(ctx)
	dataScope := int32(2)
	dataRole := client.Role.Create().
		SetName("scoped").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		SetDataScope(dataScope).
		SaveX(ctx)
	actor := client.User.Create().
		SetName("actor").
		SetPassword("hashed-password").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		SetDeptID(root.ID).
		AddRoleIDs(dataRole.ID).
		SaveX(ctx)
	peer := client.User.Create().
		SetName("peer").
		SetPassword("hashed-password").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		SetDeptID(root.ID).
		SaveX(ctx)
	childUser := client.User.Create().
		SetName("child-user").
		SetPassword("hashed-password").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		SetDeptID(child.ID).
		SaveX(ctx)
	otherUser := client.User.Create().
		SetName("other-user").
		SetPassword("hashed-password").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		SetDeptID(other.ID).
		SaveX(ctx)
	repo := NewUserRepo(&Data{db: client}, log.NewStdLogger(io.Discard))

	tests := []struct {
		name    string
		scope   int32
		custom  []uint32
		wantIDs []uint32
	}{
		{name: "self", scope: 2, wantIDs: []uint32{actor.ID}},
		{name: "department", scope: 3, wantIDs: []uint32{actor.ID, peer.ID}},
		{name: "department tree", scope: 4, wantIDs: []uint32{actor.ID, peer.ID, childUser.ID}},
		{name: "custom department", scope: 5, custom: []uint32{other.ID}, wantIDs: []uint32{actor.ID, otherUser.ID}},
		{name: "all", scope: 1, wantIDs: []uint32{actor.ID, peer.ID, childUser.ID, otherUser.ID}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			update := client.Role.UpdateOneID(dataRole.ID).
				SetDataScope(test.scope).
				ClearDataScopeDepts()
			if len(test.custom) > 0 {
				update.AddDataScopeDeptIDs(test.custom...)
			}
			update.SaveX(ctx)
			items, err := repo.ListUsers(ctx)
			if err != nil {
				t.Fatalf("ListUsers() error = %v", err)
			}
			got := make([]uint32, 0, len(items))
			for _, item := range items {
				got = append(got, item.GetId())
			}
			sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
			sort.Slice(test.wantIDs, func(i, j int) bool { return test.wantIDs[i] < test.wantIDs[j] })
			if len(got) != len(test.wantIDs) {
				t.Fatalf("visible users = %v, want %v", got, test.wantIDs)
			}
			for i := range got {
				if got[i] != test.wantIDs[i] {
					t.Fatalf("visible users = %v, want %v", got, test.wantIDs)
				}
			}
		})
	}
}

func TestUserRepoSaveRejectsMissingRequiredFieldsWithoutPanic(t *testing.T) {
	t.Parallel()

	repo := NewUserRepo(&Data{}, log.NewStdLogger(io.Discard))
	if _, err := repo.Save(context.Background(), &pbCore.User{}); err == nil {
		t.Fatal("Save() error = nil")
	}
}

func TestUserRepoRejectsCrossTenantRoleAssignment(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()
	otherRole := client.Role.Create().
		SetName("other-role").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		SaveX(tenantContext(2))
	repo := NewUserRepo(&Data{db: client}, log.NewStdLogger(io.Discard))

	_, err := repo.Save(tenantContext(1), &pbCore.User{
		Name:     ptr("tenant-one-user"),
		Password: ptr("hashed-password"),
		RoleIds:  []uint32{otherRole.ID},
	})
	if !errors.IsBadRequest(err) {
		t.Fatalf("cross-tenant role assignment error = %v, want bad request", err)
	}
}

func TestUserRepoSaveReturnsLoadedRoleMembership(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()
	ctx := tenantContext(1)
	adminRole := client.Role.Create().
		SetName("tenant-admin").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		SetIsTenantAdmin(true).
		SaveX(ctx)
	repo := NewUserRepo(&Data{db: client}, log.NewStdLogger(io.Discard))

	created, err := repo.Save(ctx, &pbCore.User{
		Name:     ptr("new-admin"),
		Password: ptr("hashed-password"),
		RoleIds:  []uint32{adminRole.ID},
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if !created.GetIsTenantAdmin() {
		t.Fatal("Save() is_tenant_admin = false, want true")
	}
	if len(created.GetRoleIds()) != 1 || created.GetRoleIds()[0] != adminRole.ID {
		t.Fatalf("Save() role_ids = %v, want [%d]", created.GetRoleIds(), adminRole.ID)
	}
}

func TestUserRepoProtectsLastEnabledTenantAdmin(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()
	ctx := tenantContext(1)
	adminRole := client.Role.Create().
		SetName("tenant-admin").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		SetIsTenantAdmin(true).
		SaveX(ctx)
	first := client.User.Create().
		SetName("first-admin").
		SetPassword("hashed-password").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		AddRoleIDs(adminRole.ID).
		SaveX(ctx)
	repo := NewUserRepo(&Data{db: client}, log.NewStdLogger(io.Discard))

	if err := repo.Delete(ctx, first.ID); !errors.IsConflict(err) {
		t.Fatalf("delete last admin error = %v, want conflict", err)
	}
	current, err := repo.FindByID(ctx, first.ID)
	if err != nil {
		t.Fatalf("find admin: %v", err)
	}
	disabled := pbEnum.Status_STATUS_DISABLED
	current.Status = &disabled
	if _, err = repo.Update(ctx, current); !errors.IsConflict(err) {
		t.Fatalf("disable last admin error = %v, want conflict", err)
	}

	client.User.Create().
		SetName("second-admin").
		SetPassword("hashed-password").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		AddRoleIDs(adminRole.ID).
		SaveX(ctx)
	if err = repo.Delete(ctx, first.ID); err != nil {
		t.Fatalf("delete admin when another remains: %v", err)
	}
	if got := client.User.Query().
		Where(
			user.StatusEQ(int32(pbEnum.Status_STATUS_ENABLED)),
			user.HasRolesWith(role.IsTenantAdminEQ(true)),
		).
		CountX(ctx); got != 1 {
		t.Fatalf("enabled admins = %d, want 1", got)
	}
}

func TestUserRepoLastAdminCountIsExplicitlyTenantScoped(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()
	adminRoleOne := client.Role.Create().
		SetName("tenant-one-admin").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		SetIsTenantAdmin(true).
		SaveX(tenantContext(1))
	adminRoleTwo := client.Role.Create().
		SetName("tenant-two-admin").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		SetIsTenantAdmin(true).
		SaveX(tenantContext(2))
	for _, name := range []string{"tenant-one-admin-a", "tenant-one-admin-b"} {
		client.User.Create().
			SetName(name).
			SetPassword("hashed-password").
			SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
			AddRoleIDs(adminRoleOne.ID).
			SaveX(tenantContext(1))
	}
	onlyTenantTwoAdmin := client.User.Create().
		SetName("tenant-two-only-admin").
		SetPassword("hashed-password").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		AddRoleIDs(adminRoleTwo.ID).
		SaveX(tenantContext(2))
	repo := NewUserRepo(&Data{db: client}, log.NewStdLogger(io.Discard))
	tenantTwoContext := tenantUserContext(2, onlyTenantTwoAdmin.ID)
	current, err := repo.FindByID(tenantTwoContext, onlyTenantTwoAdmin.ID)
	if err != nil {
		t.Fatalf("find tenant two admin: %v", err)
	}
	disabled := pbEnum.Status_STATUS_DISABLED
	current.Status = &disabled
	if _, err = repo.Update(tenantTwoContext, current); !errors.IsConflict(err) {
		t.Fatalf("disable only tenant two admin error = %v, want conflict", err)
	}
}

func TestUserRepoEnforcesTenantIsolation(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()

	repo := NewUserRepo(&Data{db: client}, log.NewStdLogger(io.Discard))
	tenantOne := tenantContext(1)
	tenantTwo := tenantContext(2)

	first, err := repo.Save(tenantOne, &pbCore.User{
		Name:     ptr("same-name"),
		Password: ptr("hashed-password"),
	})
	if err != nil {
		t.Fatalf("save tenant one user: %v", err)
	}
	if _, err := repo.Save(tenantTwo, &pbCore.User{
		Name:     ptr("same-name"),
		Password: ptr("hashed-password"),
	}); err != nil {
		t.Fatalf("save same name in tenant two: %v", err)
	}

	stored := client.User.GetX(systemContext(), first.GetId())
	if stored.TenantID != 1 {
		t.Fatalf("stored tenant_id = %d, want 1", stored.TenantID)
	}
	if _, err := repo.FindByID(tenantTwo, first.GetId()); !pb.IsUserNotFound(err) {
		t.Fatalf("FindByID() cross-tenant error = %v", err)
	}
	users, err := repo.ListUsers(tenantOne)
	if err != nil {
		t.Fatalf("list tenant one users: %v", err)
	}
	if len(users) != 1 || users[0].GetId() != first.GetId() {
		t.Fatalf("tenant one users = %#v", users)
	}
	if got := client.User.Query().Where(user.Name("same-name")).CountX(systemContext()); got != 2 {
		t.Fatalf("same-name users = %d, want 2", got)
	}
}
