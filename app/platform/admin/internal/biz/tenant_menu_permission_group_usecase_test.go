package biz

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/go-kratos/kratos/v2/log"

	pbEnum "backend-service/api/common/enum"
	pbCore "backend-service/api/core/service/v1"
	"backend-service/pkg/aip/listing"
)

type TenantMenuPermissionGroupRepoStub struct {
	current *pbCore.TenantMenuPermissionGroup
	caps    *pbCore.GetCurrentTenantCapabilitiesResponse

	saved       *pbCore.TenantMenuPermissionGroup
	updated     *pbCore.TenantMenuPermissionGroup
	deletedID   uint32
	statusID    uint32
	status      pbEnum.Status
	tenantID    uint32
	groupIDs    []uint32
	operatorID  uint32
	menuIDs     []uint32
	parentID    uint32
	versionArgs tenantGroupVersionArgs

	findErr error
}

type tenantGroupVersionArgs struct {
	tenantID    uint32
	groupID     uint32
	versionID   uint32
	autoUpgrade bool
	operatorID  uint32
}

func (r *TenantMenuPermissionGroupRepoStub) Save(_ context.Context, group *pbCore.TenantMenuPermissionGroup) (*pbCore.TenantMenuPermissionGroup, error) {
	r.saved = group
	return group, nil
}

func (r *TenantMenuPermissionGroupRepoStub) Update(_ context.Context, group *pbCore.TenantMenuPermissionGroup) (*pbCore.TenantMenuPermissionGroup, error) {
	r.updated = group
	return group, nil
}

func (r *TenantMenuPermissionGroupRepoStub) FindByID(context.Context, uint32) (*pbCore.TenantMenuPermissionGroup, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	if r.current != nil {
		return r.current, nil
	}
	return &pbCore.TenantMenuPermissionGroup{Id: 1}, nil
}

func (*TenantMenuPermissionGroupRepoStub) CountTenantMenuPermissionGroups(context.Context, ...listing.Option) (int32, error) {
	return 0, nil
}

func (*TenantMenuPermissionGroupRepoStub) ListTenantMenuPermissionGroups(context.Context, ...listing.Option) ([]*pbCore.TenantMenuPermissionGroup, error) {
	return nil, nil
}

func (r *TenantMenuPermissionGroupRepoStub) Delete(_ context.Context, id uint32) error {
	r.deletedID = id
	return nil
}

func (r *TenantMenuPermissionGroupRepoStub) UpdateStatus(_ context.Context, id uint32, status pbEnum.Status) (*pbCore.TenantMenuPermissionGroup, error) {
	r.statusID = id
	r.status = status
	return &pbCore.TenantMenuPermissionGroup{Id: id, Status: &status}, nil
}

func (*TenantMenuPermissionGroupRepoStub) ListVersions(context.Context, uint32) ([]*pbCore.TenantMenuPermissionGroupVersion, error) {
	return nil, nil
}

func (*TenantMenuPermissionGroupRepoStub) PublishVersion(context.Context, uint32, *pbCore.TenantMenuPermissionGroupVersion, string, uint32, string) (*pbCore.TenantMenuPermissionGroupVersion, error) {
	return nil, nil
}

func (*TenantMenuPermissionGroupRepoStub) RollbackVersion(context.Context, uint32, uint32, string, uint32) (*pbCore.TenantMenuPermissionGroupVersion, error) {
	return nil, nil
}

func (*TenantMenuPermissionGroupRepoStub) GetTenantGroups(context.Context, uint32) ([]*pbCore.TenantMenuPermissionGroup, error) {
	return nil, nil
}

func (*TenantMenuPermissionGroupRepoStub) GetTenantGroupBindings(context.Context, uint32) ([]*pbCore.TenantPermissionGroupBinding, error) {
	return nil, nil
}

func (r *TenantMenuPermissionGroupRepoStub) UpdateTenantGroups(_ context.Context, tenantID uint32, groupIDs []uint32, operatorID uint32) error {
	r.tenantID = tenantID
	r.groupIDs = append([]uint32(nil), groupIDs...)
	r.operatorID = operatorID
	return nil
}

func (*TenantMenuPermissionGroupRepoStub) GetTenantEffectiveMenuIDs(context.Context, uint32) ([]uint32, error) {
	return nil, nil
}

func (r *TenantMenuPermissionGroupRepoStub) GetTenantEffectiveMenus(_ context.Context, tenantID, parentID uint32) ([]*pbCore.Menu, error) {
	r.tenantID = tenantID
	r.parentID = parentID
	return []*pbCore.Menu{{Id: 100}}, nil
}

func (r *TenantMenuPermissionGroupRepoStub) GetTenantCapabilities(_ context.Context, tenantID uint32) (*pbCore.GetCurrentTenantCapabilitiesResponse, error) {
	r.tenantID = tenantID
	if r.caps != nil {
		return r.caps, nil
	}
	return &pbCore.GetCurrentTenantCapabilitiesResponse{TenantId: tenantID}, nil
}

func (r *TenantMenuPermissionGroupRepoStub) ValidateTenantMenuIDs(_ context.Context, menuIDs []uint32) error {
	r.menuIDs = append([]uint32(nil), menuIDs...)
	return nil
}

func (r *TenantMenuPermissionGroupRepoStub) UpdateTenantGroupVersion(_ context.Context, tenantID, groupID, versionID uint32, autoUpgrade bool, operatorID uint32) (*pbCore.TenantPermissionGroupBinding, error) {
	r.versionArgs = tenantGroupVersionArgs{
		tenantID:    tenantID,
		groupID:     groupID,
		versionID:   versionID,
		autoUpgrade: autoUpgrade,
		operatorID:  operatorID,
	}
	return &pbCore.TenantPermissionGroupBinding{TenantId: tenantID, GroupId: groupID}, nil
}

func TestTenantMenuPermissionGroupUsecaseChecksExistenceBeforeMutations(t *testing.T) {
	t.Parallel()

	expected := errors.New("not found")
	tests := []struct {
		name string
		run  func(*TenantMenuPermissionGroupUsecase) error
	}{
		{
			name: "update",
			run: func(uc *TenantMenuPermissionGroupUsecase) error {
				_, err := uc.Update(context.Background(), &pbCore.TenantMenuPermissionGroup{Id: 1})
				return err
			},
		},
		{
			name: "delete",
			run: func(uc *TenantMenuPermissionGroupUsecase) error {
				return uc.Delete(context.Background(), 1)
			},
		},
		{
			name: "status",
			run: func(uc *TenantMenuPermissionGroupUsecase) error {
				_, err := uc.UpdateStatus(context.Background(), 1, pbEnum.Status_STATUS_DISABLED)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &TenantMenuPermissionGroupRepoStub{findErr: expected}
			uc := NewTenantMenuPermissionGroupUsecase(repo, log.NewStdLogger(io.Discard))
			if err := tt.run(uc); !errors.Is(err, expected) {
				t.Fatalf("mutation error = %v, want %v", err, expected)
			}
			if repo.updated != nil || repo.deletedID != 0 || repo.statusID != 0 {
				t.Fatalf("mutation reached repo despite missing group: updated=%v deleted=%d status=%d", repo.updated, repo.deletedID, repo.statusID)
			}
		})
	}
}
