package data

import (
	"context"
	"io"
	"testing"

	pb "backend-service/api/avmc/admin/v1"
	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/avmc/admin/internal/data/ent/gen/user"

	"github.com/go-kratos/kratos/v2/log"
)

func TestUserRepoSaveRejectsMissingRequiredFieldsWithoutPanic(t *testing.T) {
	t.Parallel()

	repo := NewUserRepo(&Data{}, log.NewStdLogger(io.Discard))
	if _, err := repo.Save(context.Background(), &pbCore.User{}); err == nil {
		t.Fatal("Save() error = nil")
	}
}

func TestUserRepoEnforcesDomainIsolation(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()

	repo := NewUserRepo(&Data{db: client}, log.NewStdLogger(io.Discard))
	domainOne := tenantContext(1)
	domainTwo := tenantContext(2)

	first, err := repo.Save(domainOne, &pbCore.User{
		Name:     ptr("same-name"),
		Password: ptr("hashed-password"),
	})
	if err != nil {
		t.Fatalf("save domain one user: %v", err)
	}
	if _, err := repo.Save(domainTwo, &pbCore.User{
		Name:     ptr("same-name"),
		Password: ptr("hashed-password"),
	}); err != nil {
		t.Fatalf("save same name in domain two: %v", err)
	}

	stored := client.User.GetX(context.Background(), first.GetId())
	if stored.DomainID != 1 {
		t.Fatalf("stored domain_id = %d, want 1", stored.DomainID)
	}
	if _, err := repo.FindByID(domainTwo, first.GetId()); !pb.IsUserNotFound(err) {
		t.Fatalf("FindByID() cross-domain error = %v", err)
	}
	users, err := repo.ListUsers(domainOne)
	if err != nil {
		t.Fatalf("list domain one users: %v", err)
	}
	if len(users) != 1 || users[0].GetId() != first.GetId() {
		t.Fatalf("domain one users = %#v", users)
	}
	if got := client.User.Query().Where(user.Name("same-name")).CountX(context.Background()); got != 2 {
		t.Fatalf("same-name users = %d, want 2", got)
	}
}
