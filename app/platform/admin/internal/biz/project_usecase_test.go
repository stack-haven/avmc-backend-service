package biz

import (
	"context"
	stderrors "errors"
	"io"
	"testing"

	pbCore "backend-service/api/core/service/v1"
	"backend-service/pkg/aip/listing"
	"backend-service/pkg/auth/authn"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

type projectRepoStub struct {
	saveErr   error
	deleteErr error
	findErr   error

	saved       *pbCore.Project
	deletedID   uint32
	saveCalls   int
	deleteCalls int
}

func (r *projectRepoStub) Save(_ context.Context, project *pbCore.Project) (*pbCore.Project, error) {
	r.saveCalls++
	r.saved = project
	if r.saveErr != nil {
		return nil, r.saveErr
	}
	created := *project
	created.Id = 42
	return &created, nil
}

func (*projectRepoStub) Update(context.Context, *pbCore.Project) (*pbCore.Project, error) {
	return nil, nil
}

func (*projectRepoStub) UpdateStatus(context.Context, uint32, int32) error {
	return nil
}

func (r *projectRepoStub) FindByID(context.Context, uint32) (*pbCore.Project, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	return &pbCore.Project{Id: 42, Name: ptrString("project-a")}, nil
}

func (*projectRepoStub) CountProjects(context.Context, ...listing.Option) (int32, error) {
	return 0, nil
}

func (*projectRepoStub) ListProjects(context.Context, ...listing.Option) ([]*pbCore.Project, error) {
	return nil, nil
}

func (r *projectRepoStub) Delete(_ context.Context, id uint32) error {
	r.deleteCalls++
	r.deletedID = id
	return r.deleteErr
}

func ptrString(value string) *string {
	return &value
}

func projectQuotaUsecase(repo *resourceQuotaRepoStub, limit int64) *ResourceQuotaUsecase {
	return NewResourceQuotaUsecase(
		repo,
		&menuPermissionGroupRepoStub{caps: &pbCore.GetCurrentTenantCapabilitiesResponse{
			TenantId:       10,
			ResourceQuotas: map[string]int64{"projects": limit},
		}},
		log.NewStdLogger(io.Discard),
	)
}

func projectQuotaContext() context.Context {
	return authn.ContextWithAuthUser(context.Background(), resourceQuotaTestUser{subject: "7", tenant: "10"})
}

func TestProjectUsecaseCreateConsumesProjectQuota(t *testing.T) {
	t.Parallel()

	quotaRepo := &resourceQuotaRepoStub{}
	projectRepo := &projectRepoStub{}
	uc := NewProjectUsecase(projectRepo, projectQuotaUsecase(quotaRepo, 2), log.NewStdLogger(io.Discard))

	created, err := uc.Create(projectQuotaContext(), &pbCore.Project{Name: ptrString("project-a"), Code: ptrString("project-a")})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.GetId() != 42 || projectRepo.saveCalls != 1 {
		t.Fatalf("created = %v saveCalls = %d", created, projectRepo.saveCalls)
	}
	if got := quotaRepo.usages["projects"].GetUsed(); got != 1 {
		t.Fatalf("project quota used = %d, want 1", got)
	}
}

func TestProjectUsecaseCreateRejectsWhenProjectQuotaExceeded(t *testing.T) {
	t.Parallel()

	quotaRepo := &resourceQuotaRepoStub{}
	projectRepo := &projectRepoStub{}
	uc := NewProjectUsecase(projectRepo, projectQuotaUsecase(quotaRepo, 0), log.NewStdLogger(io.Discard))

	if _, err := uc.Create(projectQuotaContext(), &pbCore.Project{Name: ptrString("project-a")}); !errors.IsForbidden(err) {
		t.Fatalf("Create() error = %v, want quota forbidden", err)
	}
	if projectRepo.saveCalls != 0 {
		t.Fatalf("saveCalls = %d, want 0", projectRepo.saveCalls)
	}
}

func TestProjectUsecaseCreateReleasesQuotaOnFirstSaveFailure(t *testing.T) {
	t.Parallel()

	quotaRepo := &resourceQuotaRepoStub{}
	projectRepo := &projectRepoStub{saveErr: stderrors.New("save failed")}
	uc := NewProjectUsecase(projectRepo, projectQuotaUsecase(quotaRepo, 2), log.NewStdLogger(io.Discard))

	if _, err := uc.Create(projectQuotaContext(), &pbCore.Project{Name: ptrString("project-a")}); err == nil {
		t.Fatal("Create() error = nil, want save failure")
	}
	if got := quotaRepo.usages["projects"].GetUsed(); got != 0 {
		t.Fatalf("project quota used = %d, want 0", got)
	}
}

func TestProjectUsecaseCreateDoesNotReleaseQuotaOnReservationReplayFailure(t *testing.T) {
	t.Parallel()

	quotaRepo := &resourceQuotaRepoStub{
		replay: true,
		usages: map[string]*pbCore.TenantResourceQuotaUsage{
			"projects": {TenantId: 10, ResourceKey: "projects", Used: 1},
		},
	}
	projectRepo := &projectRepoStub{saveErr: stderrors.New("duplicate project")}
	uc := NewProjectUsecase(projectRepo, projectQuotaUsecase(quotaRepo, 2), log.NewStdLogger(io.Discard))

	if _, err := uc.Create(projectQuotaContext(), &pbCore.Project{Name: ptrString("project-a")}); err == nil {
		t.Fatal("Create() error = nil, want save failure")
	}
	if got := quotaRepo.usages["projects"].GetUsed(); got != 1 {
		t.Fatalf("project quota used = %d, want replay usage to stay 1", got)
	}
}

func TestProjectUsecaseDeleteReleasesProjectQuota(t *testing.T) {
	t.Parallel()

	quotaRepo := &resourceQuotaRepoStub{usages: map[string]*pbCore.TenantResourceQuotaUsage{
		"projects": {TenantId: 10, ResourceKey: "projects", Used: 2},
	}}
	projectRepo := &projectRepoStub{}
	uc := NewProjectUsecase(projectRepo, projectQuotaUsecase(quotaRepo, 2), log.NewStdLogger(io.Discard))

	if err := uc.Delete(projectQuotaContext(), 42); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if projectRepo.deletedID != 42 || projectRepo.deleteCalls != 1 {
		t.Fatalf("deletedID = %d deleteCalls = %d", projectRepo.deletedID, projectRepo.deleteCalls)
	}
	if got := quotaRepo.usages["projects"].GetUsed(); got != 1 {
		t.Fatalf("project quota used = %d, want 1", got)
	}
}
