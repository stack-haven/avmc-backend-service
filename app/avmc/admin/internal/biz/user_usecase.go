package biz

import (
	v1 "backend-service/api/avmc/admin/v1"
	"context"

	pbCore "backend-service/api/core/service/v1"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

var (
	// ErrUserNotFound is user not found.
	ErrUserNotFound = errors.NotFound(v1.ErrorReason_USER_NOT_FOUND.String(), "user not found")
)

// UserRepo is a Greater repo.
type UserRepo interface {
	Save(context.Context, *pbCore.User) (*pbCore.User, error)
	Update(context.Context, *pbCore.User) (*pbCore.User, error)
	FindByID(context.Context, int64) (*pbCore.User, error)
	ListByHello(context.Context, string) ([]*pbCore.User, error)
	ListAll(context.Context) ([]*pbCore.User, error)
}

// UserUsecase is a User usecase.
type UserUsecase struct {
	repo UserRepo
	log  *log.Helper
}

// NewUserUsecase new a User usecase.
func NewUserUsecase(repo UserRepo, logger log.Logger) *UserUsecase {
	return &UserUsecase{repo: repo, log: log.NewHelper(logger)}
}

// CreateUser creates a User, and returns the new User.
func (uc *UserUsecase) CreateUser(ctx context.Context, g *pbCore.User) (*pbCore.User, error) {
	uc.log.WithContext(ctx).Infof("CreateUser: %v", g.Name)
	return uc.repo.Save(ctx, g)
}
