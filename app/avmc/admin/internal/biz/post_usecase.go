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
	FindByID(context.Context, uint32) (*pbCore.Post, error)
	ListAll(context.Context) ([]*pbCore.Post, error)
	ListPage(context.Context, *pbPagination.PagingRequest) (*pbCore.ListPostResponse, error) // 新增的方法用于分页查询
	Delete(context.Context, uint32) error
}

// PostUsecase is a Post usecase.
// 包含岗位仓库和日志记录器
type PostUsecase struct {
	repo PostRepo
	log  *log.Helper
}

// NewPostUsecase new a Post usecase.
// 参数：repo 岗位仓库实例，logger 日志记录器
// 返回值：岗位用例实例指针
func NewPostUsecase(repo PostRepo, logger log.Logger) *PostUsecase {
	return &PostUsecase{repo: repo, log: log.NewHelper(logger)}
}

// Create 处理创建岗位请求
// 参数：ctx 上下文，g 岗位信息
// 返回值：创建后的岗位信息，错误信息
func (uc *PostUsecase) Create(ctx context.Context, g *pbCore.Post) (*pbCore.Post, error) {
	uc.log.WithContext(ctx).Infof("CreatePost: %v", g.Name)
	return uc.repo.Save(ctx, g)
}

// Get 处理获取岗位详情请求
// 参数：ctx 上下文，id 岗位ID
// 返回值：岗位详情，错误信息
func (uc *PostUsecase) Get(ctx context.Context, id uint32) (*pbCore.Post, error) {
	uc.log.WithContext(ctx).Infof("GetPost: %v", id)
	return uc.repo.FindByID(ctx, id)
}

// Update 处理更新岗位请求
// 参数：ctx 上下文，g 岗位信息
// 返回值：更新后的岗位信息，错误信息
func (uc *PostUsecase) Update(ctx context.Context, g *pbCore.Post) (*pbCore.Post, error) {
	uc.log.WithContext(ctx).Infof("UpdatePost: %v", g.Id)
	_, err := uc.repo.FindByID(ctx, g.GetId())
	if err != nil {
		return nil, err
	}
	return uc.repo.Update(ctx, g)
}

// ListSimple 处理获取岗位列表请求
// 参数：ctx 上下文，pageNum 页码，pageSize 每页数量
// 返回值：岗位列表，错误信息
func (uc *PostUsecase) ListSimple(ctx context.Context, pageNum, pageSize int64) ([]*pbCore.Post, error) {
	uc.log.WithContext(ctx).Infof("ListPostSimple: %v", pageNum)
	return uc.repo.ListAll(ctx)
}

// ListPage 处理分页查询岗位请求
// 参数：ctx 上下文，pagination 分页请求
// 返回值：岗位列表响应，错误信息
func (uc *PostUsecase) ListPage(ctx context.Context, pagination *pbPagination.PagingRequest) (*pbCore.ListPostResponse, error) {
	uc.log.WithContext(ctx).Infof("ListPostPage: %v", pagination)
	return uc.repo.ListPage(ctx, pagination)
}

// Delete 处理删除岗位请求
// 参数：ctx 上下文，id 岗位ID
// 返回值：错误信息
func (uc *PostUsecase) Delete(ctx context.Context, id uint32) error {
	uc.log.WithContext(ctx).Infof("DeletePost: %v", id)
	_, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	return uc.repo.Delete(ctx, id)
}
