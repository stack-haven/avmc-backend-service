package biz

import (
	v1 "backend-service/api/avmc/admin/v1"
	pbPagination "backend-service/api/common/pagination"
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
	FindByID(context.Context, uint32) (*pbCore.User, error)
	ListByName(context.Context, string) ([]*pbCore.User, error)
	ListByPhone(context.Context, string) ([]*pbCore.User, error)
	ListAll(context.Context) ([]*pbCore.User, error)
	ListPage(ctx context.Context, pagination *pbPagination.PagingRequest) (*pbCore.ListUserResponse, error)
	Delete(ctx context.Context, id uint32) error
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

func (uc *UserUsecase) Create(ctx context.Context, g *pbCore.User) (*pbCore.User, error) {
	uc.log.WithContext(ctx).Infof("CreateUser: %v", g.Name)
	return uc.repo.Save(ctx, g)
}

func (uc *UserUsecase) Get(ctx context.Context, id uint32) (*pbCore.User, error) {
	uc.log.WithContext(ctx).Infof("GetUser: %v", id)
	return uc.repo.FindByID(ctx, id)
}

func (uc *UserUsecase) Update(ctx context.Context, g *pbCore.User) (*pbCore.User, error) {
	return uc.repo.Update(ctx, g)
}

func (uc *UserUsecase) ListSimple(ctx context.Context, pageNum, pageSize int64) ([]*pbCore.User, error) {
	return uc.repo.ListAll(ctx)
}

func (uc *UserUsecase) ListPage(ctx context.Context, pagination *pbPagination.PagingRequest) (*pbCore.ListUserResponse, error) {
	return uc.repo.ListPage(ctx, pagination)
}

func (uc *UserUsecase) Delete(ctx context.Context, id uint32) error {
	return uc.repo.Delete(ctx, id)
}
