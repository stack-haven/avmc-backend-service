package data

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	pbPagination "backend-service/api/common/pagination"
	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/avmc/admin/internal/biz"
)

type menuRepo struct {
	data *Data
	log  *log.Helper
}

// NewmenuRepo .
func NewMenuRepo(data *Data, logger log.Logger) biz.MenuRepo {
	return &menuRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *menuRepo) Save(ctx context.Context, g *pbCore.Menu) (*pbCore.Menu, error) {
	return g, nil
}

func (r *menuRepo) Update(ctx context.Context, g *pbCore.Menu) (*pbCore.Menu, error) {
	return g, nil
}

func (r *menuRepo) FindByID(context.Context, int64) (*pbCore.Menu, error) {
	return nil, nil
}

func (r *menuRepo) Delete(context.Context, int64) error {
	return nil
}

func (r *menuRepo) ListByHello(context.Context, string) ([]*pbCore.Menu, error) {
	return nil, nil
}

func (r *menuRepo) ListAll(context.Context) ([]*pbCore.Menu, error) {
	return nil, nil
}

func (r *menuRepo) ListPage(ctx context.Context, pagination *pbPagination.PagingRequest) ([]*pbCore.ListMenuResponse, error) {
	return nil, nil
}
