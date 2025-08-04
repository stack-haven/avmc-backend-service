package biz

import (
	"context"

	pbPagination "backend-service/api/common/pagination"
	pbCore "backend-service/api/core/service/v1"

	"github.com/go-kratos/kratos/v2/log"
)

var (
// ErrRoleNotFound is user not found.
// ErrRoleNotFound = errors.NotFound(v1.ErrorReason_USER_NOT_FOUND.String(), "user not found")
)

// RoleRepo is a Greater repo.
type RoleRepo interface {
	Save(context.Context, *pbCore.Role) (*pbCore.Role, error)
	Update(context.Context, *pbCore.Role) (*pbCore.Role, error)
	FindByID(context.Context, uint32) (*pbCore.Role, error)
	ListAll(context.Context) ([]*pbCore.Role, error)
	ListPage(context.Context, *pbPagination.PagingRequest) (*pbCore.ListRoleResponse, error) // 新增的方法用于分页查询
	Delete(context.Context, uint32) error
}

// RoleUsecase is a Role usecase.
type RoleUsecase struct {
	repo RoleRepo
	log  *log.Helper
}

// NewRoleUsecase new a Role usecase.
func NewRoleUsecase(repo RoleRepo, logger log.Logger) *RoleUsecase {
	return &RoleUsecase{repo: repo, log: log.NewHelper(logger)}
}

func (uc *RoleUsecase) Create(ctx context.Context, g *pbCore.Role) (*pbCore.Role, error) {
	uc.log.WithContext(ctx).Infof("CreateRole: %v", g.Name)
	return uc.repo.Save(ctx, g)
}

func (uc *RoleUsecase) Get(ctx context.Context, id uint32) (*pbCore.Role, error) {
	return uc.repo.FindByID(ctx, id)
}

func (uc *RoleUsecase) Update(ctx context.Context, g *pbCore.Role) (*pbCore.Role, error) {
	return uc.repo.Update(ctx, g)
}

func (uc *RoleUsecase) ListSimple(ctx context.Context, pageNum, pageSize int64) ([]*pbCore.Role, error) {
	return uc.repo.ListAll(ctx)
}

func (uc *RoleUsecase) ListPage(ctx context.Context, pagination *pbPagination.PagingRequest) (*pbCore.ListRoleResponse, error) {
	return uc.repo.ListPage(ctx, pagination)
}

func (uc *RoleUsecase) Delete(ctx context.Context, id uint32) error {
	return uc.repo.Delete(ctx, id)
}
