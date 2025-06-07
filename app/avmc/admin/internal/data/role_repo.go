package data

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	pbPagination "backend-service/api/common/pagination"
	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/avmc/admin/internal/biz"
)

type roleRepo struct {
	data *Data
	log  *log.Helper
}

// NewroleRepo .
func NewRoleRepo(data *Data, logger log.Logger) biz.RoleRepo {
	return &roleRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *roleRepo) Save(ctx context.Context, g *pbCore.Role) (*pbCore.Role, error) {
	return g, nil
}

func (r *roleRepo) Update(ctx context.Context, g *pbCore.Role) (*pbCore.Role, error) {
	return g, nil
}

func (r *roleRepo) FindByID(context.Context, int64) (*pbCore.Role, error) {
	return nil, nil
}

func (r *roleRepo) Delete(context.Context, int64) error {
	return nil
}

func (r *roleRepo) ListByHello(context.Context, string) ([]*pbCore.Role, error) {
	return nil, nil
}

func (r *roleRepo) ListAll(context.Context) ([]*pbCore.Role, error) {
	return nil, nil
}

func (r *roleRepo) ListPage(ctx context.Context, pagination *pbPagination.PagingRequest) ([]*pbCore.ListRoleResponse, error) {
	return nil, nil
}
