package data

import (
	"context"
	"io"
	"testing"

	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/internal/data/ent/gen/user"

	"github.com/go-kratos/kratos/v2/log"
)

func TestUserRepoSaveRejectsMissingRequiredFieldsWithoutPanic(t *testing.T) {
	t.Parallel()

	repo := NewUserRepo(&Data{}, log.NewStdLogger(io.Discard))
	if _, err := repo.Save(context.Background(), &pbCore.User{}); err == nil {
		t.Fatal("Save() error = nil")
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
