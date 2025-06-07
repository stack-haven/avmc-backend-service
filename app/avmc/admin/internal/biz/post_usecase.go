package biz

import (
	"context"

	pbPagination "backend-service/api/common/pagination"
	pbCore "backend-service/api/core/service/v1"

	"github.com/go-kratos/kratos/v2/log"
)

var (
// ErrPostNotFound is user not found.
// ErrPostNotFound = errors.NotFound(v1.ErrorReason_USER_NOT_FOUND.String(), "user not found")
)

// PostRepo is a Greater repo.
type PostRepo interface {
	Save(context.Context, *pbCore.Post) (*pbCore.Post, error)
	Update(context.Context, *pbCore.Post) (*pbCore.Post, error)
	FindByID(context.Context, int64) (*pbCore.Post, error)
	ListAll(context.Context) ([]*pbCore.Post, error)
	ListPage(context.Context, *pbPagination.PagingRequest) ([]*pbCore.ListPostResponse, error) // 新增的方法用于分页查询
	Delete(context.Context, int64) error
}

// PostUsecase is a Post usecase.
type PostUsecase struct {
	repo PostRepo
	log  *log.Helper
}

// NewPostUsecase new a Post usecase.
func NewPostUsecase(repo PostRepo, logger log.Logger) *PostUsecase {
	return &PostUsecase{repo: repo, log: log.NewHelper(logger)}
}

// CreatePost creates a Post, and returns the new Post.
func (uc *PostUsecase) CreatePost(ctx context.Context, g *pbCore.Post) (*pbCore.Post, error) {
	uc.log.WithContext(ctx).Infof("CreatePost: %v", g.Name)
	return uc.repo.Save(ctx, g)
}

func (uc *PostUsecase) Create(ctx context.Context, g *pbCore.Post) (*pbCore.Post, error) {
	uc.log.WithContext(ctx).Infof("CreatePost: %v", g.Name)
	return uc.repo.Save(ctx, g)
}

func (uc *PostUsecase) Get(ctx context.Context, id int64) (*pbCore.Post, error) {
	return uc.repo.FindByID(ctx, id)
}

func (uc *PostUsecase) Update(ctx context.Context, g *pbCore.Post) (*pbCore.Post, error) {
	return uc.repo.Update(ctx, g)
}

func (uc *PostUsecase) List(ctx context.Context, pageNum, pageSize int64) ([]*pbCore.Post, error) {
	return uc.repo.ListAll(ctx)
}

func (uc *PostUsecase) ListPage(ctx context.Context, pagination *pbPagination.PagingRequest) ([]*pbCore.ListPostResponse, error) {
	return uc.repo.ListPage(ctx, pagination)
}

func (uc *PostUsecase) Delete(ctx context.Context, id int64) error {
	return uc.repo.Delete(ctx, id)
}
