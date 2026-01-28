package biz

import (
	pbEnum "backend-service/api/common/enum"
	"context"

	pbCore "backend-service/api/core/service/v1"

	"github.com/go-kratos/kratos/v2/log"
)

// UserRepo is a User repo.
type UserRepo interface {
	Save(context.Context, *pbCore.User) (*pbCore.User, error)
	Update(context.Context, *pbCore.User) (*pbCore.User, error)
	FindByID(context.Context, uint32) (*pbCore.User, error)
	ListByName(context.Context, string) ([]*pbCore.User, error)
	ListByPhone(context.Context, string) ([]*pbCore.User, error)
	ListUsers(context.Context, ...ListOption) ([]*pbCore.User, error)
	CountUsers(context.Context, ...ListOption) (int32, error)
	ListAll(context.Context) ([]*pbCore.User, error)
	ListPageSimple(context.Context, ...ListOption) ([]*pbCore.User, error)
	Delete(context.Context, uint32) error
	ExistByName(context.Context, string) (uint32, error)
	ExistByPhone(context.Context, string) (uint32, error)
	ExistByEmail(context.Context, string) (uint32, error)
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
	return uc.repo.FindByID(ctx, id)
}

// Update 处理更新用户请求
// 参数：ctx 上下文，g 用户信息
// 返回值：更新用户响应，错误信息
func (uc *UserUsecase) Update(ctx context.Context, g *pbCore.User) (*pbCore.User, error) {
	uc.log.WithContext(ctx).Infof("UpdateUser: %v", g.GetId())
	return uc.repo.Update(ctx, g)
}

// ListPageSimple 处理分页用户简单列表请求
// 参数：ctx 上下文，opts 分页选项
// 返回值：用户列表响应，错误信息
func (uc *UserUsecase) ListPageSimple(ctx context.Context, opts ...ListOption) ([]*pbCore.User, error) {
	resp, err := uc.repo.ListPageSimple(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// Delete 处理删除用户请求
// 参数：ctx 上下文，id 用户ID
// 返回值：错误信息
func (uc *UserUsecase) Delete(ctx context.Context, id uint32) error {
	return uc.repo.Delete(ctx, id)
}

// UpdateStatus 处理更新用户状态请求
// 参数：ctx 上下文，id 用户ID，status 用户状态
// 返回值：更新后的用户信息，错误信息
func (uc *UserUsecase) UpdateStatus(ctx context.Context, id uint32, status pbEnum.Status) (*pbCore.User, error) {
	uc.log.WithContext(ctx).Infof("UpdateStatus：%v %v", id, status)
	g, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	g.Status = &status
	return uc.repo.Update(ctx, g)
}

// ListUsers 处理分页用户列表请求
// 参数：ctx 上下文，opts 分页选项
// 返回值：用户列表响应，错误信息
func (uc *UserUsecase) ListUsers(ctx context.Context, opts ...ListOption) ([]*pbCore.User, error) {
	return uc.repo.ListUsers(ctx, opts...)
}

// CountUsers 处理用户条件查询聚合请求
// 参数：ctx 上下文，opts 分页选项
// 返回值：用户数量，错误信息
func (uc *UserUsecase) CountUsers(ctx context.Context, opts ...ListOption) (int32, error) {
	resp, err := uc.repo.CountUsers(ctx, opts...)
	if err != nil {
		return 0, err
	}
	return resp, nil
}
