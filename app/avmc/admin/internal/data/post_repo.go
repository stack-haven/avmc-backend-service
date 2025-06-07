package data

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	pbPagination "backend-service/api/common/pagination"
	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/avmc/admin/internal/biz"
)

type postRepo struct {
	data *Data
	log  *log.Helper
}

// NewpostRepo .
func NewPostRepo(data *Data, logger log.Logger) biz.PostRepo {
	return &postRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *postRepo) Save(ctx context.Context, g *pbCore.Post) (*pbCore.Post, error) {
	return g, nil
}

func (r *postRepo) Update(ctx context.Context, g *pbCore.Post) (*pbCore.Post, error) {
	return g, nil
}

func (r *postRepo) FindByID(context.Context, int64) (*pbCore.Post, error) {
	return nil, nil
}

func (r *postRepo) Delete(context.Context, int64) error {
	return nil
}

func (r *postRepo) ListByHello(context.Context, string) ([]*pbCore.Post, error) {
	return nil, nil
}

func (r *postRepo) ListAll(context.Context) ([]*pbCore.Post, error) {
	return nil, nil
}

func (r *postRepo) ListPage(ctx context.Context, pagination *pbPagination.PagingRequest) ([]*pbCore.ListPostResponse, error) {
	return nil, nil
}
