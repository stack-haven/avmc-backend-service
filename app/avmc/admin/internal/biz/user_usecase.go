package biz

import (
	pbPagination "backend-service/api/common/pagination"
	"context"

	pbCore "backend-service/api/core/service/v1"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

// UserRepo is a User repo.
type UserRepo interface {
	Save(context.Context, *pbCore.User) (*pbCore.User, error)
	Update(context.Context, *pbCore.User) (*pbCore.User, error)
	FindByID(context.Context, uint32) (*pbCore.User, error)
	ListByName(context.Context, string) ([]*pbCore.User, error)
	ListByPhone(context.Context, string) ([]*pbCore.User, error)
	ListAll(context.Context) ([]*pbCore.User, error)
	ListPage(context.Context, *pbPagination.PagingRequest) (*pbCore.ListUserResponse, error)
	ListPageSimple(context.Context, *pbPagination.PagingRequest) (*pbCore.ListUserResponse, error)
	Delete(context.Context, uint32) error
	GetUserExistByName(context.Context, string) (uint32, error)
	GetUserExistByPhone(context.Context, string) (uint32, error)
	GetUserExistByEmail(context.Context, string) (uint32, error)
}

// UserUsecase is a User usecase.
// 包含用户仓库和日志记录器
type UserUsecase struct {
	repo UserRepo
	log  *log.Helper
}

// NewUserUsecase new a User usecase.
func NewUserUsecase(repo UserRepo, logger log.Logger) *UserUsecase {
	return &UserUsecase{repo: repo, log: log.NewHelper(logger)}
}

// Create 处理创建用户请求
// 参数：ctx 上下文，g 用户信息
// 返回值：创建用户响应，错误信息
func (uc *UserUsecase) Create(ctx context.Context, g *pbCore.User) (*pbCore.User, error) {
	uc.log.WithContext(ctx).Infof("CreateUser: %v", g.Name)
	return uc.repo.Save(ctx, g)
}

// Get 处理获取用户详情请求
// 参数：ctx 上下文，id 用户ID
// 返回值：用户详情响应，错误信息
func (uc *UserUsecase) Get(ctx context.Context, id uint32) (*pbCore.User, error) {
	uc.log.WithContext(ctx).Infof("GetUser: %v", id)
	if id == 0 {
		return nil, errors.New(1001, "用户ID不能为空", "user id is required")
	}
	return uc.repo.FindByID(ctx, id)
}

// Update 处理更新用户请求
// 参数：ctx 上下文，g 用户信息
// 返回值：更新用户响应，错误信息
func (uc *UserUsecase) Update(ctx context.Context, g *pbCore.User) (*pbCore.User, error) {
	if g.GetId() == 0 {
		return nil, errors.New(1001, "用户ID不能为空", "user id is required")
	}
	uc.log.WithContext(ctx).Infof("UpdateUser: %v", g.GetId())
	return uc.repo.Update(ctx, g)
}

// ListPageSimple 处理分页用户简单列表请求
// 参数：ctx 上下文，pageNum 页码，pageSize 每页数量
// 返回值：用户列表响应，错误信息
func (uc *UserUsecase) ListPageSimple(ctx context.Context, pagination *pbPagination.PagingRequest) (*pbCore.ListUserResponse, error) {
	return uc.repo.ListPageSimple(ctx, pagination)
}

// ListPage 处理分页用户列表请求
// 参数：ctx 上下文，pagination 分页请求
// 返回值：用户列表响应，错误信息
func (uc *UserUsecase) ListPage(ctx context.Context, pagination *pbPagination.PagingRequest) (*pbCore.ListUserResponse, error) {
	return uc.repo.ListPage(ctx, pagination)
}

// Delete 处理删除用户请求
// 参数：ctx 上下文，id 用户ID
// 返回值：错误信息
func (uc *UserUsecase) Delete(ctx context.Context, id uint32) error {
	return uc.repo.Delete(ctx, id)
}
