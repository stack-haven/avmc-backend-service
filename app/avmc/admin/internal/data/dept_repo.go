package data

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	pbPagination "backend-service/api/common/pagination"
	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/avmc/admin/internal/biz"
)

type deptRepo struct {
	data *Data
	log  *log.Helper
}

// NewdeptRepo .
func NewDeptRepo(data *Data, logger log.Logger) biz.DeptRepo {
	return &deptRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *deptRepo) Save(ctx context.Context, g *pbCore.Dept) (*pbCore.Dept, error) {
	return g, nil
}

func (r *deptRepo) Update(ctx context.Context, g *pbCore.Dept) (*pbCore.Dept, error) {
	return g, nil
}

func (r *deptRepo) FindByID(ctx context.Context, id uint32) (*pbCore.Dept, error) {
	return nil, nil
}

func (r *deptRepo) Delete(ctx context.Context, id uint32) error {
	return nil
}

func (r *deptRepo) ListByName(ctx context.Context, name string) ([]*pbCore.Dept, error) {
	return nil, nil
}

func (r *deptRepo) ListAll(ctx context.Context) ([]*pbCore.Dept, error) {
	return nil, nil
}

func (r *deptRepo) ListPage(ctx context.Context, pagination *pbPagination.PagingRequest) ([]*pbCore.ListDeptResponse, error) {
	return nil, nil
}
