package biz

import (
	"context"
	"errors"
	"io"
	"reflect"
	"testing"

	pbEnum "backend-service/api/common/enum"
	pbCore "backend-service/api/core/service/v1"
	"backend-service/pkg/aip/listing"

	"github.com/go-kratos/kratos/v2/log"
)

type menuPermissionGroupRepoStub struct {
	current *pbCore.MenuPermissionGroup

	saved       *pbCore.MenuPermissionGroup
	updated     *pbCore.MenuPermissionGroup
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

func (r *menuPermissionGroupRepoStub) Save(_ context.Context, group *pbCore.MenuPermissionGroup) (*pbCore.MenuPermissionGroup, error) {
	r.saved = group
	return group, nil
}

func (r *menuPermissionGroupRepoStub) Update(_ context.Context, group *pbCore.MenuPermissionGroup) (*pbCore.MenuPermissionGroup, error) {
	r.updated = group
	return group, nil
}

func (r *menuPermissionGroupRepoStub) FindByID(context.Context, uint32) (*pbCore.MenuPermissionGroup, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	if r.current != nil {
		return r.current, nil
	}
	return &pbCore.MenuPermissionGroup{Id: 1}, nil
}

func (*menuPermissionGroupRepoStub) CountMenuPermissionGroups(context.Context, ...listing.Option) (int32, error) {
	return 0, nil
}

func (*menuPermissionGroupRepoStub) ListMenuPermissionGroups(context.Context, ...listing.Option) ([]*pbCore.MenuPermissionGroup, error) {
	return nil, nil
}

func (r *menuPermissionGroupRepoStub) Delete(_ context.Context, id uint32) error {
	r.deletedID = id
	return nil
}

func (r *menuPermissionGroupRepoStub) UpdateStatus(_ context.Context, id uint32, status pbEnum.Status) (*pbCore.MenuPermissionGroup, error) {
	r.statusID = id
	r.status = status
	return &pbCore.MenuPermissionGroup{Id: id, Status: &status}, nil
}

func (*menuPermissionGroupRepoStub) ListVersions(context.Context, uint32) ([]*pbCore.MenuPermissionGroupVersion, error) {
	return nil, nil
}

func (*menuPermissionGroupRepoStub) PublishVersion(context.Context, uint32, []uint32, string, uint32, string) (*pbCore.MenuPermissionGroupVersion, error) {
	return nil, nil
}

func (*menuPermissionGroupRepoStub) RollbackVersion(context.Context, uint32, uint32, string, uint32) (*pbCore.MenuPermissionGroupVersion, error) {
	return nil, nil
}

func (*menuPermissionGroupRepoStub) GetTenantGroups(context.Context, uint32) ([]*pbCore.MenuPermissionGroup, error) {
	return nil, nil
}

func (*menuPermissionGroupRepoStub) GetTenantGroupBindings(context.Context, uint32) ([]*pbCore.TenantPermissionGroupBinding, error) {
	return nil, nil
}

func (r *menuPermissionGroupRepoStub) UpdateTenantGroups(_ context.Context, tenantID uint32, groupIDs []uint32, operatorID uint32) error {
	r.tenantID = tenantID
	r.groupIDs = append([]uint32(nil), groupIDs...)
	r.operatorID = operatorID
	return nil
}

func (*menuPermissionGroupRepoStub) GetTenantEffectiveMenuIDs(context.Context, uint32) ([]uint32, error) {
	return nil, nil
}

func (r *menuPermissionGroupRepoStub) GetTenantEffectiveMenus(_ context.Context, tenantID uint32, parentID uint32) ([]*pbCore.Menu, error) {
	r.tenantID = tenantID
	r.parentID = parentID
	return []*pbCore.Menu{{Id: 100}}, nil
}

func (r *menuPermissionGroupRepoStub) ValidateTenantMenuIDs(_ context.Context, menuIDs []uint32) error {
	r.menuIDs = append([]uint32(nil), menuIDs...)
	return nil
}

func (r *menuPermissionGroupRepoStub) UpdateTenantGroupVersion(_ context.Context, tenantID, groupID, versionID uint32, autoUpgrade bool, operatorID uint32) (*pbCore.TenantPermissionGroupBinding, error) {
	r.versionArgs = tenantGroupVersionArgs{
		tenantID:    tenantID,
		groupID:     groupID,
		versionID:   versionID,
		autoUpgrade: autoUpgrade,
		operatorID:  operatorID,
	}
	return &pbCore.TenantPermissionGroupBinding{TenantId: tenantID, GroupId: groupID}, nil
}

func TestMenuPermissionGroupUsecaseChecksExistenceBeforeMutations(t *testing.T) {
	t.Parallel()

	expected := errors.New("not found")
	tests := []struct {
		name string
		run  func(*MenuPermissionGroupUsecase) error
	}{
		{
			name: "update",
			run: func(uc *MenuPermissionGroupUsecase) error {
				_, err := uc.Update(context.Background(), &pbCore.MenuPermissionGroup{Id: 1})
				return err
			},
		},
		{
			name: "delete",
			run: func(uc *MenuPermissionGroupUsecase) error {
				return uc.Delete(context.Background(), 1)
			},
		},
		{
			name: "status",
			run: func(uc *MenuPermissionGroupUsecase) error {
				_, err := uc.UpdateStatus(context.Background(), 1, pbEnum.Status_STATUS_DISABLED)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &menuPermissionGroupRepoStub{findErr: expected}
			uc := NewMenuPermissionGroupUsecase(repo, log.NewStdLogger(io.Discard))
			if err := tt.run(uc); !errors.Is(err, expected) {
				t.Fatalf("mutation error = %v, want %v", err, expected)
			}
			if repo.updated != nil || repo.deletedID != 0 || repo.statusID != 0 {
				t.Fatalf("mutation reached repo despite missing group: updated=%v deleted=%d status=%d", repo.updated, repo.deletedID, repo.statusID)
			}
		})
	}
}

func TestMenuPermissionGroupUsecaseTenantBindingsAndEffectiveMenus(t *testing.T) {
	t.Parallel()

	repo := &menuPermissionGroupRepoStub{}
	uc := NewMenuPermissionGroupUsecase(repo, log.NewStdLogger(io.Discard))

	if err := uc.UpdateTenantGroups(context.Background(), 10, []uint32{1, 2}, 7); err != nil {
		t.Fatalf("UpdateTenantGroups() error = %v", err)
	}
	if repo.tenantID != 10 || repo.operatorID != 7 || !reflect.DeepEqual(repo.groupIDs, []uint32{1, 2}) {
		t.Fatalf("tenant group update = tenant:%d groups:%v operator:%d", repo.tenantID, repo.groupIDs, repo.operatorID)
	}

	menus, err := uc.GetTenantEffectiveMenus(context.Background(), 10, 99)
	if err != nil {
		t.Fatalf("GetTenantEffectiveMenus() error = %v", err)
	}
	if repo.tenantID != 10 || repo.parentID != 99 || len(menus) != 1 || menus[0].GetId() != 100 {
		t.Fatalf("effective menus = tenant:%d parent:%d menus:%v", repo.tenantID, repo.parentID, menus)
	}

	if err := uc.ValidateTenantMenuIDs(context.Background(), []uint32{100, 101}); err != nil {
		t.Fatalf("ValidateTenantMenuIDs() error = %v", err)
	}
	if !reflect.DeepEqual(repo.menuIDs, []uint32{100, 101}) {
		t.Fatalf("validated menu ids = %v, want [100 101]", repo.menuIDs)
	}
}

func TestMenuPermissionGroupUsecaseUpdateTenantGroupVersion(t *testing.T) {
	t.Parallel()

	repo := &menuPermissionGroupRepoStub{}
	uc := NewMenuPermissionGroupUsecase(repo, log.NewStdLogger(io.Discard))

	binding, err := uc.UpdateTenantGroupVersion(context.Background(), 10, 2, 8, true, 7)
	if err != nil {
		t.Fatalf("UpdateTenantGroupVersion() error = %v", err)
	}
	if binding.GetTenantId() != 10 || binding.GetGroupId() != 2 {
		t.Fatalf("binding = %v", binding)
	}
	want := tenantGroupVersionArgs{tenantID: 10, groupID: 2, versionID: 8, autoUpgrade: true, operatorID: 7}
	if repo.versionArgs != want {
		t.Fatalf("version args = %#v, want %#v", repo.versionArgs, want)
	}
}
