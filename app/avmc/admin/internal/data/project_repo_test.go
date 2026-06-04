package data

import (
	"io"
	"strings"
	"testing"

	pb "backend-service/api/avmc/admin/v1"
	pbCore "backend-service/api/core/service/v1"

	"github.com/go-kratos/kratos/v2/log"
)

func TestProjectRepoListProjectsLoadsOwnerNames(t *testing.T) {
	ctx := tenantContext(1)
	client := newTestClient(t)
	defer client.Close()

	owner := client.User.Create().
		SetDomainID(1).
		SetName("project-owner").
		SetPassword("hashed-password").
		SaveX(ctx)
	client.Project.Create().
		SetDomainID(1).
		SetName("project-a").
		SetOwnerID(owner.ID).
		SaveX(ctx)
	client.Project.Create().
		SetDomainID(1).
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

func TestProjectRepoSavePropagatesUniquenessQueryError(t *testing.T) {
	client := newTestClient(t)
	repo := NewProjectRepo(&Data{db: client}, log.NewStdLogger(io.Discard))
	if err := client.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}

	_, err := repo.Save(tenantContext(1), &pbCore.Project{Name: ptr("project-a")})
	if err == nil || !strings.Contains(err.Error(), "checking project name uniqueness") {
		t.Fatalf("Save() error = %v, want uniqueness query error", err)
	}
}

func TestProjectRepoRejectsCrossDomainOwnerAndMember(t *testing.T) {
	ctx := tenantContext(1)
	client := newTestClient(t)
	defer client.Close()

	otherDomainUser := client.User.Create().
		SetDomainID(2).
		SetName("other-domain-user").
		SetPassword("hashed-password").
		SaveX(ctx)
	repo := NewProjectRepo(&Data{db: client}, log.NewStdLogger(io.Discard))

	if _, err := repo.Save(ctx, &pbCore.Project{
		Name:    ptr("cross-domain-owner"),
		OwnerId: ptr(otherDomainUser.ID),
	}); !pb.IsBadRequest(err) {
		t.Fatalf("cross-domain owner error = %v", err)
	}
	if _, err := repo.Save(ctx, &pbCore.Project{
		Name:      ptr("cross-domain-member"),
		MemberIds: []uint32{otherDomainUser.ID},
	}); !pb.IsBadRequest(err) {
		t.Fatalf("cross-domain member error = %v", err)
	}
}

func TestProjectRepoReturnsTypedNotFoundAcrossDomains(t *testing.T) {
	ctx := tenantContext(1)
	client := newTestClient(t)
	defer client.Close()
	project := client.Project.Create().SetDomainID(2).SetName("other-domain-project").SaveX(ctx)
	repo := NewProjectRepo(&Data{db: client}, log.NewStdLogger(io.Discard))

	if _, err := repo.FindByID(ctx, project.ID); !pb.IsProjectNotFound(err) {
		t.Fatalf("cross-domain FindByID error = %v", err)
	}
	if err := repo.Delete(ctx, project.ID); !pb.IsProjectNotFound(err) {
		t.Fatalf("cross-domain Delete error = %v", err)
	}
}
