package data

import (
	"io"
	"testing"

	pbEnum "backend-service/api/common/enum"
	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/internal/data/ent/gen/dept"

	"github.com/go-kratos/kratos/v2/errors"
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
	root, err := repo.Save(ctx, &pbCore.Dept{Name: ptr("root")})
	if err != nil {
		t.Fatalf("save root: %v", err)
	}
	parent, err := repo.Save(ctx, &pbCore.Dept{Name: ptr("parent"), ParentId: ptr(root.GetId())})
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

	root, err := repo.Save(ctx, &pbCore.Dept{Name: ptr("root")})
	if err != nil {
		t.Fatalf("save root: %v", err)
	}
	parent, err := repo.Save(ctx, &pbCore.Dept{Name: ptr("parent"), ParentId: ptr(root.GetId())})
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

func TestDeptRepoProtectsUserAndRoleReferencesOnDelete(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()
	repo := NewDeptRepo(&Data{db: client}, log.NewStdLogger(io.Discard))
	ctx := tenantContext(1)
	userDept, _ := repo.Save(ctx, &pbCore.Dept{Name: ptr("user-dept")})
	roleDept, _ := repo.Save(ctx, &pbCore.Dept{Name: ptr("role-dept")})
	client.User.Create().
		SetName("dept-user").
		SetPassword("hashed-password").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		SetDeptID(userDept.GetId()).
		SaveX(ctx)
	client.Role.Create().
		SetName("custom-scope").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		SetDataScope(5).
		AddDataScopeDeptIDs(roleDept.GetId()).
		SaveX(ctx)

	for _, id := range []uint32{userDept.GetId(), roleDept.GetId()} {
		if err := repo.Delete(ctx, id); !errors.IsConflict(err) {
			t.Fatalf("Delete(%d) error = %v, want conflict", id, err)
		}
	}
}

func TestDeptRepoMoveRewritesDescendantAncestors(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()
	repo := NewDeptRepo(&Data{db: client}, log.NewStdLogger(io.Discard))
	ctx := tenantContext(1)
	rootA, _ := repo.Save(ctx, &pbCore.Dept{Name: ptr("root-a")})
	rootB, _ := repo.Save(ctx, &pbCore.Dept{Name: ptr("root-b")})
	child, _ := repo.Save(ctx, &pbCore.Dept{Name: ptr("child"), ParentId: ptr(rootA.GetId())})
	grandchild, _ := repo.Save(ctx, &pbCore.Dept{Name: ptr("grandchild"), ParentId: ptr(child.GetId())})

	if _, err := repo.Update(ctx, &pbCore.Dept{
		Id:       child.GetId(),
		Name:     ptr("child"),
		ParentId: ptr(rootB.GetId()),
	}); err != nil {
		t.Fatalf("move child: %v", err)
	}
	stored := client.Dept.GetX(ctx, grandchild.GetId())
	want := []int{int(rootB.GetId()), int(child.GetId())}
	if len(stored.Ancestors) != len(want) {
		t.Fatalf("grandchild ancestors = %v, want %v", stored.Ancestors, want)
	}
	for i := range want {
		if stored.Ancestors[i] != want[i] {
			t.Fatalf("grandchild ancestors = %v, want %v", stored.Ancestors, want)
		}
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

func TestDeptRepoTransferAndDeleteMovesUsersAtomically(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()
	repo := NewDeptRepo(&Data{db: client}, log.NewStdLogger(io.Discard))
	ctx := tenantContext(1)
	root, _ := repo.Save(ctx, &pbCore.Dept{Name: ptr("root")})
	source, _ := repo.Save(ctx, &pbCore.Dept{Name: ptr("source"), ParentId: ptr(root.GetId())})
	target, _ := repo.Save(ctx, &pbCore.Dept{Name: ptr("target"), ParentId: ptr(root.GetId())})
	member := client.User.Create().
		SetName("transfer-user").
		SetPassword("hashed-password").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		SetDeptID(source.GetId()).
		SaveX(ctx)

	impact, err := repo.GetDeleteImpact(ctx, source.GetId())
	if err != nil {
		t.Fatalf("GetDeleteImpact() error = %v", err)
	}
	if !impact.GetRequiresUserTransfer() || impact.GetDirectUserCount() != 1 {
		t.Fatalf("impact = %+v", impact)
	}

	count, err := repo.TransferAndDelete(ctx, source.GetId(), target.GetId())
	if err != nil {
		t.Fatalf("TransferAndDelete() error = %v", err)
	}
	if count != 1 {
		t.Fatalf("transferred count = %d, want 1", count)
	}
	updated := client.User.GetX(ctx, member.ID)
	if updated.DeptID == nil || *updated.DeptID != target.GetId() {
		t.Fatalf("user dept = %v, want %d", updated.DeptID, target.GetId())
	}
	if client.Dept.Query().Where(dept.IDEQ(source.GetId())).ExistX(ctx) {
		t.Fatal("source department still visible after delete")
	}
}

func TestDeptRepoTransferAndDeleteRejectsChildrenAndCrossTenantTarget(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()
	repo := NewDeptRepo(&Data{db: client}, log.NewStdLogger(io.Discard))
	ctx := tenantContext(1)
	root, _ := repo.Save(ctx, &pbCore.Dept{Name: ptr("root")})
	source, _ := repo.Save(ctx, &pbCore.Dept{Name: ptr("source"), ParentId: ptr(root.GetId())})
	if _, err := repo.Save(ctx, &pbCore.Dept{Name: ptr("child"), ParentId: ptr(source.GetId())}); err != nil {
		t.Fatal(err)
	}
	targetTwo, _ := repo.Save(tenantContext(2), &pbCore.Dept{Name: ptr("tenant-two-root")})

	if _, err := repo.TransferAndDelete(ctx, source.GetId(), root.GetId()); !pb.IsDeptCannotDeleteWithChildren(err) {
		t.Fatalf("children error = %v", err)
	}
	leaf, _ := repo.Save(ctx, &pbCore.Dept{Name: ptr("leaf"), ParentId: ptr(root.GetId())})
	if _, err := repo.TransferAndDelete(ctx, leaf.GetId(), targetTwo.GetId()); !pb.IsBadRequest(err) {
		t.Fatalf("cross-tenant target error = %v", err)
	}
}
