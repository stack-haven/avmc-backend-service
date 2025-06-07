package biz

import (
	"context"

	pbPagination "backend-service/api/common/pagination"
	pbCore "backend-service/api/core/service/v1"

	"github.com/go-kratos/kratos/v2/log"
)

var (
// ErrDeptNotFound is user not found.
// ErrDeptNotFound = errors.NotFound(v1.ErrorReason_USER_NOT_FOUND.String(), "user not found")
)

// DeptRepo is a Greater repo.
type DeptRepo interface {
	Save(context.Context, *pbCore.Dept) (*pbCore.Dept, error)
	Update(context.Context, *pbCore.Dept) (*pbCore.Dept, error)
	FindByID(context.Context, int64) (*pbCore.Dept, error)
	ListAll(context.Context) ([]*pbCore.Dept, error)
	ListPage(context.Context, *pbPagination.PagingRequest) ([]*pbCore.ListDeptResponse, error) // 新增的方法用于分页查询
	Delete(context.Context, int64) error
}

// DeptUsecase is a Dept usecase.
type DeptUsecase struct {
	repo DeptRepo
	log  *log.Helper
}

// NewDeptUsecase new a Dept usecase.
func NewDeptUsecase(repo DeptRepo, logger log.Logger) *DeptUsecase {
	return &DeptUsecase{repo: repo, log: log.NewHelper(logger)}
}

// CreateDept creates a Dept, and returns the new Dept.
func (uc *DeptUsecase) CreateDept(ctx context.Context, g *pbCore.Dept) (*pbCore.Dept, error) {
	uc.log.WithContext(ctx).Infof("CreateDept: %v", g.Name)
	return uc.repo.Save(ctx, g)
}

func (uc *DeptUsecase) Create(ctx context.Context, g *pbCore.Dept) (*pbCore.Dept, error) {
	uc.log.WithContext(ctx).Infof("CreateDept: %v", g.Name)
	return uc.repo.Save(ctx, g)
}

func (uc *DeptUsecase) Get(ctx context.Context, id int64) (*pbCore.Dept, error) {
	return uc.repo.FindByID(ctx, id)
}

func (uc *DeptUsecase) Update(ctx context.Context, g *pbCore.Dept) (*pbCore.Dept, error) {
	return uc.repo.Update(ctx, g)
}

func (uc *DeptUsecase) List(ctx context.Context, pageNum, pageSize int64) ([]*pbCore.Dept, error) {
	return uc.repo.ListAll(ctx)
}

func (uc *DeptUsecase) ListPage(ctx context.Context, pagination *pbPagination.PagingRequest) ([]*pbCore.ListDeptResponse, error) {
	return uc.repo.ListPage(ctx, pagination)
}

func (uc *DeptUsecase) Delete(ctx context.Context, id int64) error {
	return uc.repo.Delete(ctx, id)
}
