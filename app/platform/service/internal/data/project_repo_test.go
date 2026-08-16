package data

import (
	"io"
	"sort"
	"strings"
	"testing"

	pbEnum "backend-service/api/common/enum"
	pb "backend-service/api/platform/service/v1"

	"github.com/go-kratos/kratos/v2/log"
)

func TestProjectRepoListProjectsLoadsOwnerNames(t *testing.T) {
	ctx := tenantContext(1)
	client := newTestClient(t)
	defer client.Close()

	owner := client.User.Create().
		SetTenantID(1).
		SetName("project-owner").
		SetPassword("hashed-password").
		SaveX(ctx)
	client.Project.Create().
		SetTenantID(1).
		SetName("project-a").
		SetOwnerID(owner.ID).
		SaveX(ctx)
	client.Project.Create().
		SetTenantID(1).
		SetName("project-b").
		SetOwnerID(owner.ID).
		SaveX(ctx)

	repo := NewProjectRepo(&Data{db: client}, log.NewStdLogger(io.Discard))
	projects, err := repo.ListProjects(ctx)
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("projects len = %d, want 2", len(projects))
	}
	for _, project := range projects {
		if project.GetOwnerName() != "project-owner" {
			t.Fatalf("owner name = %q, want project-owner", project.GetOwnerName())
		}
	}
}

func TestProjectRepoAppliesRoleDataScope(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()

	ctx := tenantContext(1)
	selfScope := int32(2)
	allScope := int32(1)
	selfRole := client.Role.Create().
		SetTenantID(1).
		SetName("project-self-scope").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		SetDataScope(selfScope).
		SaveX(ctx)
	allRole := client.Role.Create().
		SetTenantID(1).
		SetName("project-all-scope").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		SetDataScope(allScope).
		SaveX(ctx)
	actor := client.User.Create().
		SetTenantID(1).
		SetName("project-actor").
		SetPassword("hashed-password").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		AddRoleIDs(selfRole.ID).
		SaveX(ctx)
	allActor := client.User.Create().
		SetTenantID(1).
		SetName("project-admin").
		SetPassword("hashed-password").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		AddRoleIDs(allRole.ID).
		SaveX(ctx)
	peer := client.User.Create().
		SetTenantID(1).
		SetName("project-peer").
		SetPassword("hashed-password").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		SaveX(ctx)

	ownedByActor := client.Project.Create().
		SetTenantID(1).
		SetName("owned-by-actor").
		SetOwnerID(actor.ID).
		SaveX(ctx)
	memberActor := client.Project.Create().
		SetTenantID(1).
		SetName("member-actor").
		SetOwnerID(peer.ID).
		AddMemberIDs(actor.ID).
		SaveX(ctx)
	ownedByPeer := client.Project.Create().
		SetTenantID(1).
		SetName("owned-by-peer").
		SetOwnerID(peer.ID).
		SaveX(ctx)
	noPrincipal := client.Project.Create().
		SetTenantID(1).
		SetName("no-principal").
		SaveX(ctx)

	repo := NewProjectRepo(&Data{db: client}, log.NewStdLogger(io.Discard))
	actorCtx := tenantUserContext(1, actor.ID)
	projects, err := repo.ListProjects(actorCtx)
	if err != nil {
		t.Fatalf("ListProjects(actor) error = %v", err)
	}
	got := projectIDs(projects)
	want := []uint32{ownedByActor.ID, memberActor.ID}
	if !equalUint32s(got, want) {
		t.Fatalf("actor visible projects = %v, want %v", got, want)
	}
	count, err := repo.CountProjects(actorCtx)
	if err != nil {
		t.Fatalf("CountProjects(actor) error = %v", err)
	}
	if count != int32(len(want)) {
		t.Fatalf("CountProjects(actor) = %d, want %d", count, len(want))
	}
	if _, err = repo.FindByID(actorCtx, ownedByPeer.ID); !pb.IsProjectNotFound(err) {
		t.Fatalf("FindByID(out of scope) error = %v, want project not found", err)
	}

	allCtx := tenantUserContext(1, allActor.ID)
	projects, err = repo.ListProjects(allCtx)
	if err != nil {
		t.Fatalf("ListProjects(all) error = %v", err)
	}
	got = projectIDs(projects)
	want = []uint32{ownedByActor.ID, memberActor.ID, ownedByPeer.ID, noPrincipal.ID}
	if !equalUint32s(got, want) {
		t.Fatalf("all-scope visible projects = %v, want %v", got, want)
	}
}

func TestProjectRepoSavePropagatesUniquenessQueryError(t *testing.T) {
	client := newTestClient(t)
	repo := NewProjectRepo(&Data{db: client}, log.NewStdLogger(io.Discard))
	if err := client.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}

	_, err := repo.Save(tenantContext(1), &pb.Project{Name: ptr("project-a")})
	if err == nil || !strings.Contains(err.Error(), "checking project name uniqueness") {
		t.Fatalf("Save() error = %v, want uniqueness query error", err)
	}
}

func TestProjectRepoRejectsCrossTenantOwnerAndMember(t *testing.T) {
	ctx := tenantContext(1)
	seedCtx := systemContext()
	client := newTestClient(t)
	defer client.Close()

	otherTenantUser := client.User.Create().
		SetTenantID(2).
		SetName("other-tenant-user").
		SetPassword("hashed-password").
		SaveX(seedCtx)
	repo := NewProjectRepo(&Data{db: client}, log.NewStdLogger(io.Discard))

	if _, err := repo.Save(ctx, &pb.Project{
		Name:    ptr("cross-tenant-owner"),
		OwnerId: ptr(otherTenantUser.ID),
	}); !pb.IsBadRequest(err) {
		t.Fatalf("cross-tenant owner error = %v", err)
	}
	if _, err := repo.Save(ctx, &pb.Project{
		Name:      ptr("cross-tenant-member"),
		MemberIds: []uint32{otherTenantUser.ID},
	}); !pb.IsBadRequest(err) {
		t.Fatalf("cross-tenant member error = %v", err)
	}
}

func TestProjectMembersCanBelongToMultipleProjects(t *testing.T) {
	ctx := tenantContext(1)
	client := newTestClient(t)
	defer client.Close()

	member := client.User.Create().
		SetTenantID(1).
		SetName("shared-project-member").
		SetPassword("hashed-password").
		SaveX(ctx)
	first := client.Project.Create().
		SetTenantID(1).
		SetName("shared-member-project-a").
		AddMemberIDs(member.ID).
		SaveX(ctx)
	second := client.Project.Create().
		SetTenantID(1).
		SetName("shared-member-project-b").
		AddMemberIDs(member.ID).
		SaveX(ctx)

	if got := first.QueryMembers().CountX(ctx); got != 1 {
		t.Fatalf("first project member count = %d, want 1", got)
	}
	if got := second.QueryMembers().CountX(ctx); got != 1 {
		t.Fatalf("second project member count = %d, want 1", got)
	}
	if got := member.QueryProjects().CountX(ctx); got != 2 {
		t.Fatalf("member project count = %d, want 2", got)
	}
}

func TestProjectRepoReturnsTypedNotFoundAcrossTenants(t *testing.T) {
	ctx := tenantContext(1)
	seedCtx := systemContext()
	client := newTestClient(t)
	defer client.Close()
	project := client.Project.Create().SetTenantID(2).SetName("other-tenant-project").SaveX(seedCtx)
	repo := NewProjectRepo(&Data{db: client}, log.NewStdLogger(io.Discard))

	if _, err := repo.FindByID(ctx, project.ID); !pb.IsProjectNotFound(err) {
		t.Fatalf("cross-tenant FindByID error = %v", err)
	}
	if err := repo.Delete(ctx, project.ID); !pb.IsProjectNotFound(err) {
		t.Fatalf("cross-tenant Delete error = %v", err)
	}
}

func projectIDs(projects []*pb.Project) []uint32 {
	ids := make([]uint32, 0, len(projects))
	for _, item := range projects {
		ids = append(ids, item.GetId())
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func equalUint32s(got []uint32, want []uint32) bool {
	sort.Slice(want, func(i, j int) bool { return want[i] < want[j] })
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
