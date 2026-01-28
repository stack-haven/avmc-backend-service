package biz

import (
	"context"

	pbEnum "backend-service/api/common/enum"
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
	CountRoles(context.Context, ...ListOption) (int32, error)
	ListAll(context.Context) ([]*pbCore.Role, error)
	ListRoles(context.Context, ...ListOption) ([]*pbCore.Role, error) // 新增的方法用于分页查询
	Delete(context.Context, uint32) error
	ExistByName(context.Context, *pbCore.ExistRoleByNameRequest) (bool, error)
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

// CountRoles 处理角色条件查询聚合请求
// 参数：ctx 上下文，opts 分页选项
// 返回值：角色数量，错误信息
func (uc *RoleUsecase) CountRoles(ctx context.Context, opts ...ListOption) (int32, error) {
	resp, err := uc.repo.CountRoles(ctx, opts...)
	if err != nil {
		return 0, err
	}
	return resp, nil
}

// ListSimple 处理角色简单列表请求
// 参数：ctx 上下文，pageNum 页码，pageSize 每页数量
// 返回值：角色列表，错误信息
func (uc *RoleUsecase) ListSimple(ctx context.Context, pageNum, pageSize int64) ([]*pbCore.Role, error) {
	return uc.repo.ListAll(ctx)
}

// ListRoles 处理角色分页列表请求
// 参数：ctx 上下文，pagination 分页请求
// 返回值：角色列表响应，错误信息
func (uc *RoleUsecase) ListRoles(ctx context.Context, opts ...ListOption) ([]*pbCore.Role, error) {
	return uc.repo.ListRoles(ctx, opts...)
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

// ExistByName 处理判断菜单名是否存在请求
// 参数：ctx 上下文，req 判断菜单名是否存在请求
// 返回值：是否存在，错误信息
func (uc *RoleUsecase) ExistByName(ctx context.Context, req *pbCore.ExistRoleByNameRequest) (bool, error) {
	uc.log.WithContext(ctx).Infof("ExistByName：%v", req.GetName())
	return uc.repo.ExistByName(ctx, req)
}

// UpdateStatus 处理更新角色状态请求
// 参数：ctx 上下文，id 角色ID，status 角色状态
// 返回值：更新后的角色信息，错误信息
func (uc *RoleUsecase) UpdateStatus(ctx context.Context, id uint32, status pbEnum.Status) (*pbCore.Role, error) {
	uc.log.WithContext(ctx).Infof("UpdateStatus：%v %v", id, status)
	g, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	g.Status = &status
	return uc.repo.Update(ctx, g)
}
