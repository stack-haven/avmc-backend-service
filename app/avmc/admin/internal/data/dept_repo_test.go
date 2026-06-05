package data

import (
	"io"
	"testing"

	pb "backend-service/api/avmc/admin/v1"
	pbCore "backend-service/api/core/service/v1"

	"github.com/go-kratos/kratos/v2/log"
)

func TestDeptRepoDerivesHierarchyAndRejectsCrossTenantReferences(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()

	repo := NewDeptRepo(&Data{db: client}, log.NewStdLogger(io.Discard))
	tenantOne := tenantContext(1)
	tenantTwo := tenantContext(2)

	parent, err := repo.Save(tenantOne, &pbCore.Dept{Name: ptr("parent")})
	if err != nil {
		t.Fatalf("save parent: %v", err)
	}
	child, err := repo.Save(tenantOne, &pbCore.Dept{
		Name:      ptr("child"),
		ParentId:  ptr(parent.GetId()),
		Ancestors: []uint32{999},
	})
	if err != nil {
		t.Fatalf("save child: %v", err)
	}
	stored := client.Dept.GetX(tenantOne, child.GetId())
	if len(stored.Ancestors) != 1 || uint32(stored.Ancestors[0]) != parent.GetId() {
		t.Fatalf("stored ancestors = %v, want [%d]", stored.Ancestors, parent.GetId())
	}

	if _, err := repo.Save(tenantTwo, &pbCore.Dept{
		Name:     ptr("cross-tenant-child"),
		ParentId: ptr(parent.GetId()),
	}); !pb.IsBadRequest(err) {
		t.Fatalf("cross-tenant parent error = %v", err)
	}
	if _, err := repo.FindByID(tenantTwo, parent.GetId()); !pb.IsDeptNotFound(err) {
		t.Fatalf("cross-tenant FindByID error = %v", err)
	}
}

func TestDeptRepoRejectsHierarchyCycle(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()

	repo := NewDeptRepo(&Data{db: client}, log.NewStdLogger(io.Discard))
	ctx := tenantContext(1)
	parent, err := repo.Save(ctx, &pbCore.Dept{Name: ptr("parent")})
	if err != nil {
		t.Fatalf("save parent: %v", err)
	}
	child, err := repo.Save(ctx, &pbCore.Dept{Name: ptr("child"), ParentId: ptr(parent.GetId())})
	if err != nil {
		t.Fatalf("save child: %v", err)
	}

	if _, err := repo.Update(ctx, &pbCore.Dept{
		Id:       parent.GetId(),
		Name:     ptr("parent"),
		ParentId: ptr(child.GetId()),
	}); !pb.IsBadRequest(err) {
		t.Fatalf("cycle update error = %v", err)
	}
}

func TestDeptRepoProtectsChildrenOnDelete(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()
	repo := NewDeptRepo(&Data{db: client}, log.NewStdLogger(io.Discard))
	ctx := tenantContext(1)

	parent, err := repo.Save(ctx, &pbCore.Dept{Name: ptr("parent")})
	if err != nil {
		t.Fatalf("save parent: %v", err)
	}
	if _, err := repo.Save(ctx, &pbCore.Dept{Name: ptr("child"), ParentId: ptr(parent.GetId())}); err != nil {
		t.Fatalf("save child: %v", err)
	}
	if err := repo.Delete(ctx, parent.GetId()); !pb.IsDeptCannotDeleteWithChildren(err) {
		t.Fatalf("Delete(parent) error = %v", err)
	}
}

func TestDeptRepoListReturnsAncestors(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()
	repo := NewDeptRepo(&Data{db: client}, log.NewStdLogger(io.Discard))
	ctx := tenantContext(1)
	parent, err := repo.Save(ctx, &pbCore.Dept{Name: ptr("parent")})
	if err != nil {
		t.Fatalf("save parent: %v", err)
	}
	if _, err := repo.Save(ctx, &pbCore.Dept{Name: ptr("child"), ParentId: ptr(parent.GetId())}); err != nil {
		t.Fatalf("save child: %v", err)
	}

	items, err := repo.ListDepts(ctx)
	if err != nil {
		t.Fatalf("list depts: %v", err)
	}
	for _, item := range items {
		if item.GetName() == "child" {
			if len(item.GetAncestors()) != 1 || item.GetAncestors()[0] != parent.GetId() {
				t.Fatalf("child ancestors = %v", item.GetAncestors())
			}
			return
		}
	}
	t.Fatal("child department not found")
}
