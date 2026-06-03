package biz

import (
	"context"

	pbCore "backend-service/api/core/service/v1"

	"github.com/go-kratos/kratos/v2/log"
)

// ProjectRepo is a project repo.
type ProjectRepo interface {
	Save(context.Context, *pbCore.Project) (*pbCore.Project, error)
	Update(context.Context, *pbCore.Project) (*pbCore.Project, error)
	UpdateStatus(context.Context, uint32, int32) error
	FindByID(context.Context, uint32) (*pbCore.Project, error)
	CountProjects(context.Context, ...ListOption) (int32, error)
	ListProjects(context.Context, ...ListOption) ([]*pbCore.Project, error)
	Delete(context.Context, uint32) error
}

// ProjectUsecase is a project usecase.
type ProjectUsecase struct {
	repo ProjectRepo
	log  *log.Helper
}

// NewProjectUsecase creates a project usecase.
func NewProjectUsecase(repo ProjectRepo, logger log.Logger) *ProjectUsecase {
	return &ProjectUsecase{repo: repo, log: log.NewHelper(logger)}
}

// Create creates a project.
func (uc *ProjectUsecase) Create(ctx context.Context, g *pbCore.Project) (*pbCore.Project, error) {
	uc.log.WithContext(ctx).Infof("CreateProject: %v", g.GetName())
	return uc.repo.Save(ctx, g)
}

// Get gets a project by id.
func (uc *ProjectUsecase) Get(ctx context.Context, id uint32) (*pbCore.Project, error) {
	uc.log.WithContext(ctx).Infof("GetProject: %v", id)
	return uc.repo.FindByID(ctx, id)
}

// Update updates a project.
func (uc *ProjectUsecase) Update(ctx context.Context, g *pbCore.Project) (*pbCore.Project, error) {
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
func (uc *ProjectUsecase) CountProjects(ctx context.Context, opts ...ListOption) (int32, error) {
	return uc.repo.CountProjects(ctx, opts...)
}

// ListProjects lists projects by list options.
func (uc *ProjectUsecase) ListProjects(ctx context.Context, opts ...ListOption) ([]*pbCore.Project, error) {
	uc.log.WithContext(ctx).Infof("ListProjects: %v", opts)
	return uc.repo.ListProjects(ctx, opts...)
}

// Delete deletes a project.
func (uc *ProjectUsecase) Delete(ctx context.Context, id uint32) error {
	uc.log.WithContext(ctx).Infof("DeleteProject: %v", id)
	_, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	return uc.repo.Delete(ctx, id)
}
