package biz

import (
	"context"

	pbPagination "backend-service/api/common/pagination"
	pbCore "backend-service/api/core/service/v1"

	"github.com/go-kratos/kratos/v2/log"
)

var (
// ErrMenuNotFound is user not found.
// ErrMenuNotFound = errors.NotFound(v1.ErrorReason_USER_NOT_FOUND.String(), "user not found")
)

// MenuRepo is a Greater repo.
type MenuRepo interface {
	Save(context.Context, *pbCore.Menu) (*pbCore.Menu, error)
	Update(context.Context, *pbCore.Menu) (*pbCore.Menu, error)
	FindByID(context.Context, uint32) (*pbCore.Menu, error)
	ListAll(context.Context) ([]*pbCore.Menu, error)
	ListPage(context.Context, *pbPagination.PagingRequest) ([]*pbCore.ListMenuResponse, error) // 新增的方法用于分页查询
	Delete(context.Context, uint32) error
}

// MenuUsecase is a Menu usecase.
type MenuUsecase struct {
	repo MenuRepo
	log  *log.Helper
}

// NewMenuUsecase new a Menu usecase.
func NewMenuUsecase(repo MenuRepo, logger log.Logger) *MenuUsecase {
	return &MenuUsecase{repo: repo, log: log.NewHelper(logger)}
}

func (uc *MenuUsecase) Create(ctx context.Context, g *pbCore.Menu) (*pbCore.Menu, error) {
	uc.log.WithContext(ctx).Infof("CreateMenu: %v", g.Name)
	return uc.repo.Save(ctx, g)
}

func (uc *MenuUsecase) Get(ctx context.Context, id uint32) (*pbCore.Menu, error) {
	return uc.repo.FindByID(ctx, id)
}

func (uc *MenuUsecase) Update(ctx context.Context, g *pbCore.Menu) (*pbCore.Menu, error) {
	return uc.repo.Update(ctx, g)
}

func (uc *MenuUsecase) ListSimple(ctx context.Context, pageNum, pageSize int64) ([]*pbCore.Menu, error) {
	return uc.repo.ListAll(ctx)
}

func (uc *MenuUsecase) ListPage(ctx context.Context, pagination *pbPagination.PagingRequest) ([]*pbCore.ListMenuResponse, error) {
	return uc.repo.ListPage(ctx, pagination)
}

func (uc *MenuUsecase) Delete(ctx context.Context, id uint32) error {
	return uc.repo.Delete(ctx, id)
}
