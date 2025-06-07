package data

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/avmc/admin/internal/biz"
)

type userRepo struct {
	data *Data
	log  *log.Helper
}

// NewuserRepo .
func NewUserRepo(data *Data, logger log.Logger) biz.UserRepo {
	return &userRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *userRepo) Save(ctx context.Context, g *pbCore.User) (*pbCore.User, error) {
	return g, nil
}

func (r *userRepo) Update(ctx context.Context, g *pbCore.User) (*pbCore.User, error) {
	return g, nil
}

func (r *userRepo) FindByID(context.Context, int64) (*pbCore.User, error) {
	return nil, nil
}

func (r *userRepo) ListByHello(context.Context, string) ([]*pbCore.User, error) {
	return nil, nil
}

func (r *userRepo) ListAll(context.Context) ([]*pbCore.User, error) {
	return nil, nil
}
