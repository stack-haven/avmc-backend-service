package biz

import (
	pbEnum "backend-service/api/common/enum"
	pbCore "backend-service/api/core/service/v1"
	"backend-service/pkg/aip/listing"
	"context"

	"github.com/go-kratos/kratos/v2/log"
)

// MenuPermissionGroupRepo manages reusable tenant menu permission groups.
type MenuPermissionGroupRepo interface {
	Save(context.Context, *pbCore.MenuPermissionGroup) (*pbCore.MenuPermissionGroup, error)
	Update(context.Context, *pbCore.MenuPermissionGroup) (*pbCore.MenuPermissionGroup, error)
	FindByID(context.Context, uint32) (*pbCore.MenuPermissionGroup, error)
	CountMenuPermissionGroups(context.Context, ...listing.Option) (int32, error)
	ListMenuPermissionGroups(context.Context, ...listing.Option) ([]*pbCore.MenuPermissionGroup, error)
	Delete(context.Context, uint32) error
	UpdateStatus(context.Context, uint32, pbEnum.Status) (*pbCore.MenuPermissionGroup, error)
	GetTenantGroups(context.Context, uint32) ([]*pbCore.MenuPermissionGroup, error)
	UpdateTenantGroups(context.Context, uint32, []uint32, uint32) error
	GetTenantEffectiveMenuIDs(context.Context, uint32) ([]uint32, error)
	GetTenantEffectiveMenus(context.Context, uint32, uint32) ([]*pbCore.Menu, error)
	ValidateTenantMenuIDs(context.Context, []uint32) error
}

// MenuPermissionGroupUsecase contains tenant menu permission group business rules.
type MenuPermissionGroupUsecase struct {
	repo MenuPermissionGroupRepo
	log  *log.Helper
}

func NewMenuPermissionGroupUsecase(repo MenuPermissionGroupRepo, logger log.Logger) *MenuPermissionGroupUsecase {
	return &MenuPermissionGroupUsecase{repo: repo, log: log.NewHelper(logger)}
}

func (uc *MenuPermissionGroupUsecase) Create(ctx context.Context, group *pbCore.MenuPermissionGroup) (*pbCore.MenuPermissionGroup, error) {
	uc.log.WithContext(ctx).Infof("CreateMenuPermissionGroup: %s", group.GetCode())
	return uc.repo.Save(ctx, group)
}

func (uc *MenuPermissionGroupUsecase) Update(ctx context.Context, group *pbCore.MenuPermissionGroup) (*pbCore.MenuPermissionGroup, error) {
	uc.log.WithContext(ctx).Infof("UpdateMenuPermissionGroup: %d", group.GetId())
	if _, err := uc.repo.FindByID(ctx, group.GetId()); err != nil {
		return nil, err
	}
	return uc.repo.Update(ctx, group)
}

func (uc *MenuPermissionGroupUsecase) Get(ctx context.Context, id uint32) (*pbCore.MenuPermissionGroup, error) {
	return uc.repo.FindByID(ctx, id)
}

func (uc *MenuPermissionGroupUsecase) Count(ctx context.Context, opts ...listing.Option) (int32, error) {
	return uc.repo.CountMenuPermissionGroups(ctx, opts...)
}

func (uc *MenuPermissionGroupUsecase) List(ctx context.Context, opts ...listing.Option) ([]*pbCore.MenuPermissionGroup, error) {
	return uc.repo.ListMenuPermissionGroups(ctx, opts...)
}

func (uc *MenuPermissionGroupUsecase) Delete(ctx context.Context, id uint32) error {
	if _, err := uc.repo.FindByID(ctx, id); err != nil {
		return err
	}
	return uc.repo.Delete(ctx, id)
}

func (uc *MenuPermissionGroupUsecase) UpdateStatus(ctx context.Context, id uint32, status pbEnum.Status) (*pbCore.MenuPermissionGroup, error) {
	if _, err := uc.repo.FindByID(ctx, id); err != nil {
		return nil, err
	}
	return uc.repo.UpdateStatus(ctx, id, status)
}

func (uc *MenuPermissionGroupUsecase) GetTenantGroups(ctx context.Context, tenantID uint32) ([]*pbCore.MenuPermissionGroup, error) {
	return uc.repo.GetTenantGroups(ctx, tenantID)
}

func (uc *MenuPermissionGroupUsecase) UpdateTenantGroups(ctx context.Context, tenantID uint32, groupIDs []uint32, operatorID uint32) error {
	return uc.repo.UpdateTenantGroups(ctx, tenantID, groupIDs, operatorID)
}

func (uc *MenuPermissionGroupUsecase) GetTenantEffectiveMenus(ctx context.Context, tenantID uint32, parentID uint32) ([]*pbCore.Menu, error) {
	return uc.repo.GetTenantEffectiveMenus(ctx, tenantID, parentID)
}

func (uc *MenuPermissionGroupUsecase) ValidateTenantMenuIDs(ctx context.Context, menuIDs []uint32) error {
	return uc.repo.ValidateTenantMenuIDs(ctx, menuIDs)
}
