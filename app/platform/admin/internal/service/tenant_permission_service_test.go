package service

import (
	"context"
	"io"
	"reflect"
	"testing"

	pbEnum "backend-service/api/common/enum"
	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/pkg/aip/listing"
	"backend-service/pkg/auth/authn"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

type tenantPermissionRepoStub struct {
	groups   []*pbCore.MenuPermissionGroup
	bindings []*pbCore.TenantPermissionGroupBinding
	menus    []*pbCore.Menu
	caps     *pbCore.GetCurrentTenantCapabilitiesResponse
	binding  *pbCore.TenantPermissionGroupBinding

	tenantID    uint32
	parentID    uint32
	groupIDs    []uint32
	operatorID  uint32
	versionArgs serviceTenantGroupVersionArgs
}

type serviceTenantGroupVersionArgs struct {
	tenantID    uint32
	groupID     uint32
	versionID   uint32
	autoUpgrade bool
	operatorID  uint32
}

func (*tenantPermissionRepoStub) Save(context.Context, *pbCore.MenuPermissionGroup) (*pbCore.MenuPermissionGroup, error) {
	return nil, nil
}
func (*tenantPermissionRepoStub) Update(context.Context, *pbCore.MenuPermissionGroup) (*pbCore.MenuPermissionGroup, error) {
	return nil, nil
}
func (*tenantPermissionRepoStub) FindByID(context.Context, uint32) (*pbCore.MenuPermissionGroup, error) {
	return nil, nil
}
func (*tenantPermissionRepoStub) CountMenuPermissionGroups(context.Context, ...listing.Option) (int32, error) {
	return 0, nil
}
func (*tenantPermissionRepoStub) ListMenuPermissionGroups(context.Context, ...listing.Option) ([]*pbCore.MenuPermissionGroup, error) {
	return nil, nil
}
func (*tenantPermissionRepoStub) Delete(context.Context, uint32) error { return nil }
func (*tenantPermissionRepoStub) UpdateStatus(context.Context, uint32, pbEnum.Status) (*pbCore.MenuPermissionGroup, error) {
	return nil, nil
}
func (*tenantPermissionRepoStub) ListVersions(context.Context, uint32) ([]*pbCore.MenuPermissionGroupVersion, error) {
	return nil, nil
}
func (*tenantPermissionRepoStub) PublishVersion(context.Context, uint32, *pbCore.MenuPermissionGroupVersion, string, uint32, string) (*pbCore.MenuPermissionGroupVersion, error) {
	return nil, nil
}
func (*tenantPermissionRepoStub) RollbackVersion(context.Context, uint32, uint32, string, uint32) (*pbCore.MenuPermissionGroupVersion, error) {
	return nil, nil
}
func (r *tenantPermissionRepoStub) GetTenantGroups(_ context.Context, tenantID uint32) ([]*pbCore.MenuPermissionGroup, error) {
	r.tenantID = tenantID
	return r.groups, nil
}
func (r *tenantPermissionRepoStub) GetTenantGroupBindings(_ context.Context, tenantID uint32) ([]*pbCore.TenantPermissionGroupBinding, error) {
	r.tenantID = tenantID
	return r.bindings, nil
}
func (r *tenantPermissionRepoStub) UpdateTenantGroups(_ context.Context, tenantID uint32, groupIDs []uint32, operatorID uint32) error {
	r.tenantID = tenantID
	r.groupIDs = append([]uint32(nil), groupIDs...)
	r.operatorID = operatorID
	return nil
}
func (*tenantPermissionRepoStub) GetTenantEffectiveMenuIDs(context.Context, uint32) ([]uint32, error) {
	return nil, nil
}
func (r *tenantPermissionRepoStub) GetTenantEffectiveMenus(_ context.Context, tenantID uint32, parentID uint32) ([]*pbCore.Menu, error) {
	r.tenantID = tenantID
	r.parentID = parentID
	return r.menus, nil
}
func (r *tenantPermissionRepoStub) GetTenantCapabilities(_ context.Context, tenantID uint32) (*pbCore.GetCurrentTenantCapabilitiesResponse, error) {
	r.tenantID = tenantID
	if r.caps != nil {
		return r.caps, nil
	}
	return &pbCore.GetCurrentTenantCapabilitiesResponse{TenantId: tenantID}, nil
}
func (*tenantPermissionRepoStub) ValidateTenantMenuIDs(context.Context, []uint32) error { return nil }
func (r *tenantPermissionRepoStub) UpdateTenantGroupVersion(_ context.Context, tenantID, groupID, versionID uint32, autoUpgrade bool, operatorID uint32) (*pbCore.TenantPermissionGroupBinding, error) {
	r.versionArgs = serviceTenantGroupVersionArgs{
		tenantID:    tenantID,
		groupID:     groupID,
		versionID:   versionID,
		autoUpgrade: autoUpgrade,
		operatorID:  operatorID,
	}
	if r.binding != nil {
		return r.binding, nil
	}
	return &pbCore.TenantPermissionGroupBinding{TenantId: tenantID, GroupId: groupID}, nil
}

type tenantPermissionTestUser struct {
	subject string
	tenant  string
}

func (u tenantPermissionTestUser) Name() string                           { return "test" }
func (u tenantPermissionTestUser) ParseFromContext(context.Context) error { return nil }
func (u tenantPermissionTestUser) GetSubject() string                     { return u.subject }
func (u tenantPermissionTestUser) GetObject() string                      { return "" }
func (u tenantPermissionTestUser) GetAction() string                      { return "" }
func (u tenantPermissionTestUser) GetTenant() string                      { return u.tenant }

func newTenantPermissionService(repo *tenantPermissionRepoStub) *TenantPermissionServiceService {
	uc := biz.NewMenuPermissionGroupUsecase(repo, log.NewStdLogger(io.Discard))
	quota := biz.NewResourceQuotaUsecase(&serviceResourceQuotaRepoStub{}, repo, log.NewStdLogger(io.Discard))
	return NewTenantPermissionServiceService(uc, quota, log.NewStdLogger(io.Discard))
}

type serviceResourceQuotaRepoStub struct {
	usage *pbCore.TenantResourceQuotaUsage
}

func (*serviceResourceQuotaRepoStub) ListUsage(context.Context, uint32) ([]*pbCore.TenantResourceQuotaUsage, error) {
	return nil, nil
}

func (r *serviceResourceQuotaRepoStub) GetUsage(_ context.Context, tenantID uint32, resourceKey string) (*pbCore.TenantResourceQuotaUsage, error) {
	if r.usage != nil {
		return r.usage, nil
	}
	return &pbCore.TenantResourceQuotaUsage{TenantId: tenantID, ResourceKey: resourceKey}, nil
}

func (r *serviceResourceQuotaRepoStub) Consume(_ context.Context, tenantID uint32, resourceKey string, amount int64, limit int64, unlimited bool, _ uint32) (*pbCore.TenantResourceQuotaUsage, error) {
	usage := &pbCore.TenantResourceQuotaUsage{TenantId: tenantID, ResourceKey: resourceKey, Used: amount}
	if r.usage != nil {
		usage.Used += r.usage.GetUsed()
	}
	r.usage = usage
	return usage, nil
}

func (r *serviceResourceQuotaRepoStub) Release(_ context.Context, tenantID uint32, resourceKey string, amount int64, _ uint32) (*pbCore.TenantResourceQuotaUsage, error) {
	used := int64(0)
	if r.usage != nil {
		used = r.usage.GetUsed() - amount
	}
	if used < 0 {
		used = 0
	}
	r.usage = &pbCore.TenantResourceQuotaUsage{TenantId: tenantID, ResourceKey: resourceKey, Used: used}
	return r.usage, nil
}

func TestTenantPermissionServiceGetGroupsReturnsIDsAndBindings(t *testing.T) {
	t.Parallel()

	enabled := true
	repo := &tenantPermissionRepoStub{
		groups: []*pbCore.MenuPermissionGroup{
			{Id: 1},
			nil,
			{Id: 2},
		},
		bindings: []*pbCore.TenantPermissionGroupBinding{{TenantId: 10, GroupId: 1, Enabled: &enabled}},
	}
	service := newTenantPermissionService(repo)

	resp, err := service.GetTenantPermissionGroups(context.Background(), &pbCore.GetTenantPermissionGroupsRequest{TenantId: 10})
	if err != nil {
		t.Fatalf("GetTenantPermissionGroups() error = %v", err)
	}
	if repo.tenantID != 10 || !reflect.DeepEqual(resp.GetGroupIds(), []uint32{1, 2}) || len(resp.GetBindings()) != 1 {
		t.Fatalf("response tenant=%d groupIDs=%v bindings=%v", repo.tenantID, resp.GetGroupIds(), resp.GetBindings())
	}
}

func TestTenantPermissionServiceRejectsMissingTenantID(t *testing.T) {
	t.Parallel()

	service := newTenantPermissionService(&tenantPermissionRepoStub{})
	calls := []struct {
		name string
		run  func() error
	}{
		{
			name: "get groups",
			run: func() error {
				_, err := service.GetTenantPermissionGroups(context.Background(), &pbCore.GetTenantPermissionGroupsRequest{})
				return err
			},
		},
		{
			name: "update groups",
			run: func() error {
				_, err := service.UpdateTenantPermissionGroups(context.Background(), &pbCore.UpdateTenantPermissionGroupsRequest{})
				return err
			},
		},
		{
			name: "effective menus",
			run: func() error {
				_, err := service.GetTenantEffectiveMenus(context.Background(), &pbCore.GetTenantEffectiveMenusRequest{})
				return err
			},
		},
	}
	for _, call := range calls {
		t.Run(call.name, func(t *testing.T) {
			if err := call.run(); !errors.IsBadRequest(err) {
				t.Fatalf("error = %v, want bad request", err)
			}
		})
	}
}

func TestTenantPermissionServiceUpdatesGroupsAndVersions(t *testing.T) {
	t.Parallel()

	operatorID := uint32(7)
	versionID := uint32(8)
	repo := &tenantPermissionRepoStub{}
	service := newTenantPermissionService(repo)

	if _, err := service.UpdateTenantPermissionGroups(context.Background(), &pbCore.UpdateTenantPermissionGroupsRequest{
		TenantId:   10,
		GroupIds:   []uint32{1, 2},
		OperatorId: &operatorID,
	}); err != nil {
		t.Fatalf("UpdateTenantPermissionGroups() error = %v", err)
	}
	if repo.tenantID != 10 || repo.operatorID != 7 || !reflect.DeepEqual(repo.groupIDs, []uint32{1, 2}) {
		t.Fatalf("group update = tenant:%d groups:%v operator:%d", repo.tenantID, repo.groupIDs, repo.operatorID)
	}

	resp, err := service.UpdateTenantPermissionGroupVersion(context.Background(), &pbCore.UpdateTenantPermissionGroupVersionRequest{
		TenantId:    10,
		GroupId:     2,
		VersionId:   &versionID,
		AutoUpgrade: true,
		OperatorId:  &operatorID,
	})
	if err != nil {
		t.Fatalf("UpdateTenantPermissionGroupVersion() error = %v", err)
	}
	want := serviceTenantGroupVersionArgs{tenantID: 10, groupID: 2, versionID: 8, autoUpgrade: true, operatorID: 7}
	if repo.versionArgs != want || resp.GetBinding().GetTenantId() != 10 {
		t.Fatalf("version args=%#v response=%v", repo.versionArgs, resp.GetBinding())
	}
}

func TestTenantPermissionServiceEffectiveMenus(t *testing.T) {
	t.Parallel()

	parentID := uint32(99)
	repo := &tenantPermissionRepoStub{menus: []*pbCore.Menu{{Id: 100}}}
	service := newTenantPermissionService(repo)

	resp, err := service.GetTenantEffectiveMenus(context.Background(), &pbCore.GetTenantEffectiveMenusRequest{TenantId: 10, ParentId: &parentID})
	if err != nil {
		t.Fatalf("GetTenantEffectiveMenus() error = %v", err)
	}
	if repo.tenantID != 10 || repo.parentID != 99 || len(resp.GetItems()) != 1 {
		t.Fatalf("tenant=%d parent=%d items=%v", repo.tenantID, repo.parentID, resp.GetItems())
	}
}

func TestTenantPermissionServiceCurrentTenantEffectiveMenus(t *testing.T) {
	t.Parallel()

	parentID := uint32(99)
	repo := &tenantPermissionRepoStub{menus: []*pbCore.Menu{{Id: 100}}}
	service := newTenantPermissionService(repo)
	ctx := authn.ContextWithAuthUser(context.Background(), tenantPermissionTestUser{subject: "7", tenant: "10"})

	resp, err := service.GetCurrentTenantEffectiveMenus(ctx, &pbCore.GetCurrentTenantEffectiveMenusRequest{ParentId: &parentID})
	if err != nil {
		t.Fatalf("GetCurrentTenantEffectiveMenus() error = %v", err)
	}
	if repo.tenantID != 10 || repo.parentID != 99 || len(resp.GetItems()) != 1 {
		t.Fatalf("tenant=%d parent=%d items=%v", repo.tenantID, repo.parentID, resp.GetItems())
	}

	if _, err := service.GetCurrentTenantEffectiveMenus(context.Background(), &pbCore.GetCurrentTenantEffectiveMenusRequest{}); !errors.IsBadRequest(err) {
		t.Fatalf("missing tenant context error = %v, want bad request", err)
	}
}

func TestTenantPermissionServiceCurrentTenantCapabilities(t *testing.T) {
	t.Parallel()

	repo := &tenantPermissionRepoStub{
		caps: &pbCore.GetCurrentTenantCapabilitiesResponse{
			TenantId:       10,
			ApiPermissions: []string{"platform.user.list"},
			FeatureFlags:   map[string]bool{"advanced_reports": true},
			ResourceQuotas: map[string]int64{"projects": 10},
			GroupIds:       []uint32{1},
		},
	}
	service := newTenantPermissionService(repo)
	ctx := authn.ContextWithAuthUser(context.Background(), tenantPermissionTestUser{subject: "7", tenant: "10"})

	resp, err := service.GetCurrentTenantCapabilities(ctx, &pbCore.GetCurrentTenantCapabilitiesRequest{})
	if err != nil {
		t.Fatalf("GetCurrentTenantCapabilities() error = %v", err)
	}
	if repo.tenantID != 10 || resp.GetTenantId() != 10 || len(resp.GetApiPermissions()) != 1 {
		t.Fatalf("tenant=%d response=%v", repo.tenantID, resp)
	}

	if _, err := service.GetCurrentTenantCapabilities(context.Background(), &pbCore.GetCurrentTenantCapabilitiesRequest{}); !errors.IsBadRequest(err) {
		t.Fatalf("missing tenant context error = %v, want bad request", err)
	}
}

func TestTenantPermissionServiceCurrentTenantResourceQuotas(t *testing.T) {
	t.Parallel()

	repo := &tenantPermissionRepoStub{
		caps: &pbCore.GetCurrentTenantCapabilitiesResponse{
			TenantId:       10,
			ResourceQuotas: map[string]int64{"projects": 5},
		},
	}
	service := newTenantPermissionService(repo)
	ctx := authn.ContextWithAuthUser(context.Background(), tenantPermissionTestUser{subject: "7", tenant: "10"})

	check, err := service.CheckCurrentTenantResourceQuota(ctx, &pbCore.CheckCurrentTenantResourceQuotaRequest{
		ResourceKey: "projects",
		Amount:      3,
	})
	if err != nil {
		t.Fatalf("CheckCurrentTenantResourceQuota() error = %v", err)
	}
	if !check.GetAllowed() || check.GetUsage().GetLimit() != 5 {
		t.Fatalf("quota check = %v", check)
	}

	consumed, err := service.ConsumeCurrentTenantResourceQuota(ctx, &pbCore.ConsumeCurrentTenantResourceQuotaRequest{
		ResourceKey: "projects",
		Amount:      3,
	})
	if err != nil {
		t.Fatalf("ConsumeCurrentTenantResourceQuota() error = %v", err)
	}
	if consumed.GetUsage().GetUsed() != 3 || consumed.GetUsage().GetRemaining() != 2 {
		t.Fatalf("quota consume = %v", consumed)
	}

	released, err := service.ReleaseCurrentTenantResourceQuota(ctx, &pbCore.ReleaseCurrentTenantResourceQuotaRequest{
		ResourceKey: "projects",
		Amount:      2,
	})
	if err != nil {
		t.Fatalf("ReleaseCurrentTenantResourceQuota() error = %v", err)
	}
	if released.GetUsage().GetUsed() != 1 || released.GetUsage().GetRemaining() != 4 {
		t.Fatalf("quota release = %v", released)
	}
}
