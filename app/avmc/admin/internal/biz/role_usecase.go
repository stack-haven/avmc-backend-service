package biz

import (
	"context"

	pbPagination "backend-service/api/common/pagination"
	pbCore "backend-service/api/core/service/v1"

	"github.com/go-kratos/kratos/v2/log"
)

var (
// ErrRoleNotFound is user not found.
// ErrRoleNotFound = errors.NotFound(v1.ErrorReason_USER_NOT_FOUND.String(), "user not found")
)

// RoleRepo is a Greater repo.
type RoleRepo interface {
	Save(context.Context, *pbCore.Role) (*pbCore.Role, error)
	Update(context.Context, *pbCore.Role) (*pbCore.Role, error)
	FindByID(context.Context, uint32) (*pbCore.Role, error)
	ListAll(context.Context) ([]*pbCore.Role, error)
	ListPage(context.Context, *pbPagination.PagingRequest) (*pbCore.ListRoleResponse, error) // 新增的方法用于分页查询
	Delete(context.Context, uint32) error
}

// RoleUsecase is a Role usecase.
// 包含角色仓库和日志记录器
type RoleUsecase struct {
	repo RoleRepo
	log  *log.Helper
}

// NewRoleUsecase new a Role usecase.

func NewRoleUsecase(repo RoleRepo, logger log.Logger) *RoleUsecase {
	return &RoleUsecase{repo: repo, log: log.NewHelper(logger)}
}

// Create 处理创建角色请求
// 参数：ctx 上下文，g 角色信息
// 返回值：创建后的角色信息，错误信息
func (uc *RoleUsecase) Create(ctx context.Context, g *pbCore.Role) (*pbCore.Role, error) {
	uc.log.WithContext(ctx).Infof("CreateRole: %v", g.Name)
	return uc.repo.Save(ctx, g)
}

// Get 处理获取角色详情请求
// 参数：ctx 上下文，id 角色ID
// 返回值：角色详情，错误信息
func (uc *RoleUsecase) Get(ctx context.Context, id uint32) (*pbCore.Role, error) {
	return uc.repo.FindByID(ctx, id)
}

// Update 处理更新角色请求
// 参数：ctx 上下文，g 角色信息
// 返回值：更新后的角色信息，错误信息
func (uc *RoleUsecase) Update(ctx context.Context, g *pbCore.Role) (*pbCore.Role, error) {
	uc.log.WithContext(ctx).Infof("UpdateRole: %v", g.Name)
	_, err := uc.repo.FindByID(ctx, g.GetId())
	if err != nil {
		return nil, err
	}
	return uc.repo.Update(ctx, g)
}

// ListSimple 处理角色简单列表请求
// 参数：ctx 上下文，pageNum 页码，pageSize 每页数量
// 返回值：角色列表，错误信息
func (uc *RoleUsecase) ListSimple(ctx context.Context, pageNum, pageSize int64) ([]*pbCore.Role, error) {
	return uc.repo.ListAll(ctx)
}

// ListPage 处理角色分页列表请求
// 参数：ctx 上下文，pagination 分页请求
// 返回值：角色列表响应，错误信息
func (uc *RoleUsecase) ListPage(ctx context.Context, pagination *pbPagination.PagingRequest) (*pbCore.ListRoleResponse, error) {
	return uc.repo.ListPage(ctx, pagination)
}

// Delete 处理删除角色请求
// 参数：ctx 上下文，id 角色ID
// 返回值：错误信息
func (uc *RoleUsecase) Delete(ctx context.Context, id uint32) error {
	uc.log.WithContext(ctx).Infof("DeleteRole: %v", id)
	_, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	return uc.repo.Delete(ctx, id)
}
