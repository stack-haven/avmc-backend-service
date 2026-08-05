package biz

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	pbEnum "backend-service/api/common/enum"
	pbCore "backend-service/api/core/service/v1"
	"backend-service/pkg/aip/listing"
)

// TenantMenuPermissionGroupRepo manages reusable tenant menu permission groups.
type TenantMenuPermissionGroupRepo interface {
	Save(context.Context, *pbCore.TenantMenuPermissionGroup) (*pbCore.TenantMenuPermissionGroup, error)
	Update(context.Context, *pbCore.TenantMenuPermissionGroup) (*pbCore.TenantMenuPermissionGroup, error)
	FindByID(context.Context, uint32) (*pbCore.TenantMenuPermissionGroup, error)
	CountTenantMenuPermissionGroups(context.Context, ...listing.Option) (int32, error)
	ListTenantMenuPermissionGroups(context.Context, ...listing.Option) ([]*pbCore.TenantMenuPermissionGroup, error)
	Delete(context.Context, uint32) error
	UpdateStatus(context.Context, uint32, pbEnum.Status) (*pbCore.TenantMenuPermissionGroup, error)
	ListVersions(context.Context, uint32) ([]*pbCore.TenantMenuPermissionGroupVersion, error)
	PublishVersion(context.Context, uint32, *pbCore.TenantMenuPermissionGroupVersion, string, uint32, string) (*pbCore.TenantMenuPermissionGroupVersion, error)
	RollbackVersion(context.Context, uint32, uint32, string, uint32) (*pbCore.TenantMenuPermissionGroupVersion, error)
}

// TenantMenuPermissionGroupUsecase contains menu permission group business rules.
type TenantMenuPermissionGroupUsecase struct {
	repo TenantMenuPermissionGroupRepo
	log  *log.Helper
}

func NewTenantMenuPermissionGroupUsecase(repo TenantMenuPermissionGroupRepo, logger log.Logger) *TenantMenuPermissionGroupUsecase {
	return &TenantMenuPermissionGroupUsecase{repo: repo, log: log.NewHelper(logger)}
}

func (uc *TenantMenuPermissionGroupUsecase) ListVersions(ctx context.Context, groupID uint32) ([]*pbCore.TenantMenuPermissionGroupVersion, error) {
	return uc.repo.ListVersions(ctx, groupID)
}

func (uc *TenantMenuPermissionGroupUsecase) PublishVersion(ctx context.Context, groupID uint32, version *pbCore.TenantMenuPermissionGroupVersion, summary string, operatorID uint32, effectiveAt string) (*pbCore.TenantMenuPermissionGroupVersion, error) {
	return uc.repo.PublishVersion(ctx, groupID, version, summary, operatorID, effectiveAt)
}

func (uc *TenantMenuPermissionGroupUsecase) RollbackVersion(ctx context.Context, groupID, sourceVersionID uint32, summary string, operatorID uint32) (*pbCore.TenantMenuPermissionGroupVersion, error) {
	return uc.repo.RollbackVersion(ctx, groupID, sourceVersionID, summary, operatorID)
}

func (uc *TenantMenuPermissionGroupUsecase) Create(ctx context.Context, group *pbCore.TenantMenuPermissionGroup) (*pbCore.TenantMenuPermissionGroup, error) {
	uc.log.WithContext(ctx).Infof("CreateTenantMenuPermissionGroup: %s", group.GetCode())
	return uc.repo.Save(ctx, group)
}

func (uc *TenantMenuPermissionGroupUsecase) Update(ctx context.Context, group *pbCore.TenantMenuPermissionGroup) (*pbCore.TenantMenuPermissionGroup, error) {
	uc.log.WithContext(ctx).Infof("UpdateTenantMenuPermissionGroup: %d", group.GetId())
	if _, err := uc.repo.FindByID(ctx, group.GetId()); err != nil {
		return nil, err
	}
	return uc.repo.Update(ctx, group)
}

func (uc *TenantMenuPermissionGroupUsecase) Get(ctx context.Context, id uint32) (*pbCore.TenantMenuPermissionGroup, error) {
	return uc.repo.FindByID(ctx, id)
}

func (uc *TenantMenuPermissionGroupUsecase) Count(ctx context.Context, opts ...listing.Option) (int32, error) {
	return uc.repo.CountTenantMenuPermissionGroups(ctx, opts...)
}

func (uc *TenantMenuPermissionGroupUsecase) List(ctx context.Context, opts ...listing.Option) ([]*pbCore.TenantMenuPermissionGroup, error) {
	return uc.repo.ListTenantMenuPermissionGroups(ctx, opts...)
}

func (uc *TenantMenuPermissionGroupUsecase) Delete(ctx context.Context, id uint32) error {
	if _, err := uc.repo.FindByID(ctx, id); err != nil {
		return err
	}
	return uc.repo.Delete(ctx, id)
}

func (uc *TenantMenuPermissionGroupUsecase) UpdateStatus(ctx context.Context, id uint32, status pbEnum.Status) (*pbCore.TenantMenuPermissionGroup, error) {
	if _, err := uc.repo.FindByID(ctx, id); err != nil {
		return nil, err
	}
	return uc.repo.UpdateStatus(ctx, id, status)
}
