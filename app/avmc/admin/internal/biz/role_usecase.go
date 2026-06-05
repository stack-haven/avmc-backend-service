package biz

import (
	pbEnum "backend-service/api/common/enum"
	"backend-service/pkg/aip/listing"
	"context"

	pbCore "backend-service/api/core/service/v1"

	"github.com/go-kratos/kratos/v2/log"
)

// RoleRepo is a Role repo.
type RoleRepo interface {
	Save(context.Context, *pbCore.Role) (*pbCore.Role, error)
	Update(context.Context, *pbCore.Role) (*pbCore.Role, error)
	FindByID(context.Context, uint32) (*pbCore.Role, error)
	CountRoles(context.Context, ...listing.Option) (int32, error)
	ListAll(context.Context) ([]*pbCore.Role, error)
	ListRoles(context.Context, ...listing.Option) ([]*pbCore.Role, error)
	Delete(context.Context, uint32) error
	ExistByName(ctx context.Context, name string, excludeID uint32) (bool, error)
}

// RoleUsecase 角色业务逻辑
type RoleUsecase struct {
	repo RoleRepo
	log  *log.Helper
}

// NewRoleUsecase new a Role usecase.
func NewRoleUsecase(repo RoleRepo, logger log.Logger) *RoleUsecase {
	return &RoleUsecase{repo: repo, log: log.NewHelper(logger)}
}

// Create 创建角色
func (uc *RoleUsecase) Create(ctx context.Context, g *pbCore.Role) (*pbCore.Role, error) {
	uc.log.WithContext(ctx).Infof("CreateRole: %v", g.GetName())
	return uc.repo.Save(ctx, g)
}

// Get 获取角色
func (uc *RoleUsecase) Get(ctx context.Context, id uint32) (*pbCore.Role, error) {
	return uc.repo.FindByID(ctx, id)
}

// Update 更新角色 — 先验证存在再更新
func (uc *RoleUsecase) Update(ctx context.Context, g *pbCore.Role) (*pbCore.Role, error) {
	uc.log.WithContext(ctx).Infof("UpdateRole: %v", g.GetId())
	if _, err := uc.repo.FindByID(ctx, g.GetId()); err != nil {
		return nil, err
	}
	return uc.repo.Update(ctx, g)
}

// CountRoles 角色计数
func (uc *RoleUsecase) CountRoles(ctx context.Context, opts ...listing.Option) (int32, error) {
	return uc.repo.CountRoles(ctx, opts...)
}

// ListSimple 角色简单列表
func (uc *RoleUsecase) ListSimple(ctx context.Context) ([]*pbCore.Role, error) {
	return uc.repo.ListAll(ctx)
}

// ListRoles 角色分页列表
func (uc *RoleUsecase) ListRoles(ctx context.Context, opts ...listing.Option) ([]*pbCore.Role, error) {
	return uc.repo.ListRoles(ctx, opts...)
}

// Delete 删除角色
func (uc *RoleUsecase) Delete(ctx context.Context, id uint32) error {
	uc.log.WithContext(ctx).Infof("DeleteRole: %v", id)
	if _, err := uc.repo.FindByID(ctx, id); err != nil {
		return err
	}
	return uc.repo.Delete(ctx, id)
}

// ExistByName 判断角色名是否存在
func (uc *RoleUsecase) ExistByName(ctx context.Context, name string, excludeID uint32) (bool, error) {
	uc.log.WithContext(ctx).Infof("ExistByName: %v", name)
	return uc.repo.ExistByName(ctx, name, excludeID)
}

// UpdateStatus 更新角色状态
func (uc *RoleUsecase) UpdateStatus(ctx context.Context, id uint32, status int32) (*pbCore.Role, error) {
	uc.log.WithContext(ctx).Infof("UpdateStatus: %d %d", id, status)
	g, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	s := pbEnum.Status(status)
	g.Status = &s
	return uc.repo.Update(ctx, g)
}
