package biz

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/platform/service/v1"

	"backend-service/pkg/aip/listing"
)

// ProjectRepo is a project repo.
type ProjectRepo interface {
	Save(context.Context, *pb.Project) (*pb.Project, error)
	Update(context.Context, *pb.Project) (*pb.Project, error)
	UpdateStatus(context.Context, uint32, int32) error
	FindByID(context.Context, uint32) (*pb.Project, error)
	CountProjects(context.Context, ...listing.Option) (int32, error)
	ListProjects(context.Context, ...listing.Option) ([]*pb.Project, error)
	Delete(context.Context, uint32) error
}

// ProjectUsecase is a project usecase.
type ProjectUsecase struct {
	repo  ProjectRepo
	quota *ResourceQuotaUsecase
	log   *log.Helper
}

// NewProjectUsecase creates a project usecase.
func NewProjectUsecase(repo ProjectRepo, quota *ResourceQuotaUsecase, logger log.Logger) *ProjectUsecase {
	return &ProjectUsecase{repo: repo, quota: quota, log: log.NewHelper(logger)}
}

// Create creates a project.
func (uc *ProjectUsecase) Create(ctx context.Context, g *pb.Project) (*pb.Project, error) {
	uc.log.WithContext(ctx).Infof("CreateProject: %v", g.GetName())
	if uc.quota == nil {
		return uc.repo.Save(ctx, g)
	}
	reservation, _, err := uc.quota.ReserveCurrent(ctx, projectResourceKey, 1, projectCreateQuotaKey(g))
	if err != nil {
		return nil, err
	}
	created, err := uc.repo.Save(ctx, g)
	if err != nil {
		if !reservation.IsReplay() {
			if _, releaseErr := reservation.Release(ctx); releaseErr != nil {
				uc.log.WithContext(ctx).Warnf("release project quota after create failure: %v", releaseErr)
			}
		}
		return nil, err
	}
	return created, nil
}

// Get gets a project by id.
func (uc *ProjectUsecase) Get(ctx context.Context, id uint32) (*pb.Project, error) {
	uc.log.WithContext(ctx).Infof("GetProject: %v", id)
	return uc.repo.FindByID(ctx, id)
}

// Update updates a project.
func (uc *ProjectUsecase) Update(ctx context.Context, g *pb.Project) (*pb.Project, error) {
	uc.log.WithContext(ctx).Infof("UpdateProject: %v", g.GetId())
	_, err := uc.repo.FindByID(ctx, g.GetId())
	if err != nil {
		return nil, err
	}
	return uc.repo.Update(ctx, g)
}

// UpdateStatus updates a project status.
func (uc *ProjectUsecase) UpdateStatus(ctx context.Context, id uint32, status int32) error {
	uc.log.WithContext(ctx).Infof("UpdateProjectStatus: %v", id)
	_, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	return uc.repo.UpdateStatus(ctx, id, status)
}

// CountProjects counts projects by list options.
func (uc *ProjectUsecase) CountProjects(ctx context.Context, opts ...listing.Option) (int32, error) {
	return uc.repo.CountProjects(ctx, opts...)
}

// ListProjects lists projects by list options.
func (uc *ProjectUsecase) ListProjects(ctx context.Context, opts ...listing.Option) ([]*pb.Project, error) {
	uc.log.WithContext(ctx).Infof("ListProjects")
	return uc.repo.ListProjects(ctx, opts...)
}

// Delete deletes a project.
func (uc *ProjectUsecase) Delete(ctx context.Context, id uint32) error {
	uc.log.WithContext(ctx).Infof("DeleteProject: %v", id)
	_, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if err := uc.repo.Delete(ctx, id); err != nil {
		return err
	}
	if uc.quota == nil {
		return nil
	}
	if _, err = uc.quota.ReleaseCurrent(ctx, projectResourceKey, 1, projectDeleteQuotaKey(id)); err != nil {
		uc.log.WithContext(ctx).Warnf("release project quota after delete: %v", err)
		return err
	}
	return nil
}

const projectResourceKey = "projects"

func projectCreateQuotaKey(g *pb.Project) string {
	if g == nil {
		return "project:create:unknown"
	}
	identity := g.GetCode()
	if identity == "" {
		identity = g.GetName()
	}
	if identity == "" {
		identity = "unknown"
	}
	return "project:create:" + identity
}

func projectDeleteQuotaKey(id uint32) string {
	return fmt.Sprintf("project:delete:%d", id)
}
