package data

import (
	"context"
	"io"
	"testing"

	pb "backend-service/api/platform/service/v1"

	"github.com/go-kratos/kratos/v2/log"
)

func TestMenuRepoListSimpleDoesNotPanicWhenStatusWasNotSelected(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	defer client.Close()

	client.Menu.Create().SetName("system").SetTitle("System").SaveX(ctx)
	repo := NewMenuRepo(&Data{db: client}, log.NewStdLogger(io.Discard)).(*menuRepo)
	menus, err := repo.ListAllSimple(ctx)
	if err != nil {
		t.Fatalf("list simple menus: %v", err)
	}
	if len(menus) != 1 || menus[0].GetName() != "system" {
		t.Fatalf("menus = %#v", menus)
	}
}

func TestMenuRepoRejectsInvalidInputWithoutPanic(t *testing.T) {
	repo := NewMenuRepo(&Data{}, log.NewStdLogger(io.Discard)).(*menuRepo)
	if _, err := repo.Save(context.Background(), nil); !pb.IsMenuNameCannotBeEmpty(err) {
		t.Fatalf("Save(nil) error = %v", err)
	}
	if _, err := repo.Update(context.Background(), &pb.Menu{}); !pb.IsMenuInvalidId(err) {
		t.Fatalf("Update(empty) error = %v", err)
	}
}

func TestMenuRepoReturnsNotFoundAndProtectsChildren(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	defer client.Close()
	repo := NewMenuRepo(&Data{db: client}, log.NewStdLogger(io.Discard)).(*menuRepo)

	if _, err := repo.FindByID(ctx, 999); !pb.IsMenuNotFound(err) {
		t.Fatalf("FindByID(missing) error = %v", err)
	}
	parent := client.Menu.Create().SetName("parent").SetTitle("Parent").SaveX(ctx)
	client.Menu.Create().SetName("child").SetTitle("Child").SetParentID(parent.ID).SaveX(ctx)
	if err := repo.Delete(ctx, parent.ID); !pb.IsMenuCannotDeleteWithChildren(err) {
		t.Fatalf("Delete(parent) error = %v", err)
	}
}

func TestMenuRepoRejectsDuplicateNameAndPath(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	defer client.Close()
	repo := NewMenuRepo(&Data{db: client}, log.NewStdLogger(io.Discard)).(*menuRepo)
	path := "/users"

	if _, err := repo.Save(ctx, &pb.Menu{Name: "users", Path: &path, Meta: &pb.MenuMeta{Title: ptr("Users")}}); err != nil {
		t.Fatalf("save first menu: %v", err)
	}
	if _, err := repo.Save(ctx, &pb.Menu{Name: "users", Path: ptr("/other"), Meta: &pb.MenuMeta{Title: ptr("Other")}}); !pb.IsMenuNameAlreadyExists(err) {
		t.Fatalf("duplicate name error = %v", err)
	}
	if _, err := repo.Save(ctx, &pb.Menu{Name: "other", Path: &path, Meta: &pb.MenuMeta{Title: ptr("Other")}}); !pb.IsMenuPathAlreadyExists(err) {
		t.Fatalf("duplicate path error = %v", err)
	}
}

func TestMenuRepoRejectsInvalidParentAndCycle(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)
	defer client.Close()
	repo := NewMenuRepo(&Data{db: client}, log.NewStdLogger(io.Discard)).(*menuRepo)

	if _, err := repo.Save(ctx, &pb.Menu{Name: "orphan", ParentId: ptr(uint32(999))}); !pb.IsBadRequest(err) {
		t.Fatalf("missing parent error = %v", err)
	}
	parent := client.Menu.Create().SetName("parent").SetTitle("Parent").SaveX(ctx)
	child := client.Menu.Create().SetName("child").SetTitle("Child").SetParentID(parent.ID).SaveX(ctx)
	if _, err := repo.Update(ctx, &pb.Menu{Id: parent.ID, Name: "parent", ParentId: &child.ID}); !pb.IsBadRequest(err) {
		t.Fatalf("cycle error = %v", err)
	}
}
