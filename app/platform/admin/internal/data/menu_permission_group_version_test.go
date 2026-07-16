package data

import (
	pbEnum "backend-service/api/common/enum"
	pbCore "backend-service/api/core/service/v1"
	"context"
	"io"
	"reflect"
	"testing"

	"backend-service/app/platform/admin/internal/data/ent/gen/tenantpermissiongroup"

	"github.com/go-kratos/kratos/v2/log"
)

func TestMenuPermissionGroupVersionPublishPinAndRollback(t *testing.T) {
	ctx := systemContext()
	client := newTestClient(t)
	defer client.Close()

	firstMenu := client.Menu.Create().SetName("version-menu-1").SetTitle("Version 1").SetStatus(1).SaveX(ctx)
	secondMenu := client.Menu.Create().SetName("version-menu-2").SetTitle("Version 2").SetStatus(1).SaveX(ctx)
	client.Tenant.Create().SetID(101).SetName("Auto Tenant").SetCode("auto-tenant").SetStatus(1).SaveX(ctx)
	client.Tenant.Create().SetID(102).SetName("Pinned Tenant").SetCode("pinned-tenant").SetStatus(1).SaveX(ctx)

	repo := NewMenuPermissionGroupRepo(&Data{db: client}, log.NewStdLogger(io.Discard))
	status := pbEnum.Status_STATUS_ENABLED
	group, err := repo.Save(ctx, &pbCore.MenuPermissionGroup{
		Name:    ptr("Versioned Package"),
		Code:    ptr("versioned-package"),
		Status:  &status,
		MenuIds: []uint32{firstMenu.ID},
	})
	if err != nil {
		t.Fatalf("create versioned package: %v", err)
	}
	if group.GetCurrentVersion() != 1 || group.GetCurrentVersionId() == 0 {
		t.Fatalf("initial version = %d/%d, want version 1", group.GetCurrentVersionId(), group.GetCurrentVersion())
	}
	if _, err = repo.Update(ctx, &pbCore.MenuPermissionGroup{
		Id:          group.GetId(),
		Name:        ptr("Versioned Package Renamed"),
		Code:        ptr("versioned-package"),
		Description: ptr("metadata only"),
	}); err != nil {
		t.Fatalf("metadata-only update: %v", err)
	}
	metadataVersions, err := repo.ListVersions(ctx, group.GetId())
	if err != nil {
		t.Fatalf("list versions after metadata update: %v", err)
	}
	if len(metadataVersions) != 1 || !sameUint32Set(metadataVersions[0].GetMenuIds(), []uint32{firstMenu.ID}) {
		t.Fatalf("metadata update changed permission versions: %#v", metadataVersions)
	}
	if err = repo.UpdateTenantGroups(ctx, 101, []uint32{group.GetId()}, 0); err != nil {
		t.Fatalf("bind auto tenant: %v", err)
	}
	if err = repo.UpdateTenantGroups(ctx, 102, []uint32{group.GetId()}, 0); err != nil {
		t.Fatalf("bind pinned tenant: %v", err)
	}
	if _, err = repo.UpdateTenantGroupVersion(ctx, 102, group.GetId(), group.GetCurrentVersionId(), false, 0); err != nil {
		t.Fatalf("pin tenant version: %v", err)
	}
	if err = repo.UpdateTenantGroups(ctx, 102, []uint32{group.GetId()}, 9); err != nil {
		t.Fatalf("resave pinned tenant packages: %v", err)
	}
	preserved := client.TenantPermissionGroup.Query().
		Where(tenantpermissiongroup.TenantIDEQ(102), tenantpermissiongroup.GroupIDEQ(group.GetId())).
		OnlyX(ctx)
	if preserved.AutoUpgrade || preserved.VersionID == nil || *preserved.VersionID != group.GetCurrentVersionId() {
		t.Fatalf("resaving package selection reset pinned version: %#v", preserved)
	}

	version2, err := repo.PublishVersion(ctx, group.GetId(), &pbCore.MenuPermissionGroupVersion{MenuIds: []uint32{secondMenu.ID}}, "replace menu", 7, "")
	if err != nil {
		t.Fatalf("publish version 2: %v", err)
	}
	if version2.GetVersion() != 2 || len(version2.GetMenuIds()) != 1 || version2.GetMenuIds()[0] != secondMenu.ID {
		t.Fatalf("version 2 = %#v", version2)
	}
	autoBinding := client.TenantPermissionGroup.Query().
		Where(tenantpermissiongroup.TenantIDEQ(101), tenantpermissiongroup.GroupIDEQ(group.GetId())).
		OnlyX(ctx)
	pinnedBinding := client.TenantPermissionGroup.Query().
		Where(tenantpermissiongroup.TenantIDEQ(102), tenantpermissiongroup.GroupIDEQ(group.GetId())).
		OnlyX(ctx)
	if autoBinding.VersionID == nil || *autoBinding.VersionID != version2.GetId() {
		t.Fatalf("auto tenant version = %v, want %d", autoBinding.VersionID, version2.GetId())
	}
	if pinnedBinding.VersionID == nil || *pinnedBinding.VersionID != group.GetCurrentVersionId() || pinnedBinding.AutoUpgrade {
		t.Fatalf("pinned tenant binding changed: %#v", pinnedBinding)
	}
	assertMenuIDs(t, repo, ctx, 101, []uint32{secondMenu.ID})
	assertMenuIDs(t, repo, ctx, 102, []uint32{firstMenu.ID})

	version3, err := repo.RollbackVersion(ctx, group.GetId(), group.GetCurrentVersionId(), "rollback", 7)
	if err != nil {
		t.Fatalf("rollback to version 1: %v", err)
	}
	if version3.GetVersion() != 3 || len(version3.GetMenuIds()) != 1 || version3.GetMenuIds()[0] != firstMenu.ID {
		t.Fatalf("rollback version = %#v", version3)
	}
	assertMenuIDs(t, repo, ctx, 101, []uint32{firstMenu.ID})
	assertMenuIDs(t, repo, ctx, 102, []uint32{firstMenu.ID})
}

func TestMenuPermissionGroupCapabilityPackageVersionSnapshots(t *testing.T) {
	ctx := systemContext()
	client := newTestClient(t)
	defer client.Close()

	firstMenu := client.Menu.Create().SetName("capability-menu-1").SetTitle("Capability 1").SetStatus(1).SaveX(ctx)
	secondMenu := client.Menu.Create().SetName("capability-menu-2").SetTitle("Capability 2").SetStatus(1).SaveX(ctx)

	repo := NewMenuPermissionGroupRepo(&Data{db: client}, log.NewStdLogger(io.Discard))
	status := pbEnum.Status_STATUS_ENABLED
	group, err := repo.Save(ctx, &pbCore.MenuPermissionGroup{
		Name:           ptr("Capability Package"),
		Code:           ptr("capability-package"),
		Status:         &status,
		MenuIds:        []uint32{firstMenu.ID},
		ApiPermissions: []string{"platform.user.list", "platform.user.list", "platform.audit.read"},
		FeatureFlags: map[string]bool{
			"advanced_reports": true,
			"webhook_center":   false,
		},
		ResourceQuotas: map[string]int64{
			"projects": 10,
			"storage":  1024,
		},
	})
	if err != nil {
		t.Fatalf("create capability package: %v", err)
	}
	assertPackageCapabilities(t, group, []string{"platform.audit.read", "platform.user.list"}, map[string]bool{
		"advanced_reports": true,
		"webhook_center":   false,
	}, map[string]int64{"projects": 10, "storage": 1024})

	initialVersions, err := repo.ListVersions(ctx, group.GetId())
	if err != nil {
		t.Fatalf("list initial capability versions: %v", err)
	}
	if len(initialVersions) != 1 {
		t.Fatalf("initial version count = %d, want 1", len(initialVersions))
	}
	assertVersionCapabilities(t, initialVersions[0], []uint32{firstMenu.ID}, []string{"platform.audit.read", "platform.user.list"}, map[string]bool{
		"advanced_reports": true,
		"webhook_center":   false,
	}, map[string]int64{"projects": 10, "storage": 1024})

	published, err := repo.PublishVersion(ctx, group.GetId(), &pbCore.MenuPermissionGroupVersion{
		MenuIds:        []uint32{secondMenu.ID},
		ApiPermissions: []string{"platform.audit.read"},
		FeatureFlags: map[string]bool{
			"advanced_reports": false,
			"webhook_center":   true,
		},
		ResourceQuotas: map[string]int64{
			"projects": 25,
			"storage":  4096,
		},
	}, "capability upgrade", 7, "")
	if err != nil {
		t.Fatalf("publish capability version: %v", err)
	}
	if published.GetVersion() != 2 {
		t.Fatalf("published version = %d, want 2", published.GetVersion())
	}
	assertVersionCapabilities(t, published, []uint32{secondMenu.ID}, []string{"platform.audit.read"}, map[string]bool{
		"advanced_reports": false,
		"webhook_center":   true,
	}, map[string]int64{"projects": 25, "storage": 4096})

	current, err := repo.FindByID(ctx, group.GetId())
	if err != nil {
		t.Fatalf("find current capability package: %v", err)
	}
	assertPackageCapabilities(t, current, []string{"platform.audit.read"}, map[string]bool{
		"advanced_reports": false,
		"webhook_center":   true,
	}, map[string]int64{"projects": 25, "storage": 4096})

	_, err = repo.Update(ctx, &pbCore.MenuPermissionGroup{
		Id:               group.GetId(),
		Name:             ptr("Capability Package"),
		Code:             ptr("capability-package"),
		ApiPermissions:   []string{"platform.audit.read", "platform.billing.read"},
		FeatureFlags:     current.GetFeatureFlags(),
		ResourceQuotas:   current.GetResourceQuotas(),
		Description:      ptr("capability change without menu change"),
		MenuIds:          nil,
		CurrentVersion:   current.CurrentVersion,
		CurrentVersionId: current.CurrentVersionId,
	})
	if err != nil {
		t.Fatalf("update capability package: %v", err)
	}
	afterCapabilityUpdate, err := repo.ListVersions(ctx, group.GetId())
	if err != nil {
		t.Fatalf("list versions after capability update: %v", err)
	}
	if len(afterCapabilityUpdate) != 3 || afterCapabilityUpdate[0].GetVersion() != 3 {
		t.Fatalf("versions after capability update = %#v", afterCapabilityUpdate)
	}
	assertVersionCapabilities(t, afterCapabilityUpdate[0], []uint32{secondMenu.ID}, []string{"platform.audit.read", "platform.billing.read"}, current.GetFeatureFlags(), current.GetResourceQuotas())

	rollback, err := repo.RollbackVersion(ctx, group.GetId(), initialVersions[0].GetId(), "rollback capability", 7)
	if err != nil {
		t.Fatalf("rollback capability version: %v", err)
	}
	if rollback.GetVersion() != 4 {
		t.Fatalf("rollback version = %d, want 4", rollback.GetVersion())
	}
	assertVersionCapabilities(t, rollback, []uint32{firstMenu.ID}, []string{"platform.audit.read", "platform.user.list"}, map[string]bool{
		"advanced_reports": true,
		"webhook_center":   false,
	}, map[string]int64{"projects": 10, "storage": 1024})
}

func assertMenuIDs(t *testing.T, repo interface {
	GetTenantEffectiveMenuIDs(context.Context, uint32) ([]uint32, error)
}, ctx context.Context, tenantID uint32, want []uint32) {
	t.Helper()
	got, err := repo.GetTenantEffectiveMenuIDs(ctx, tenantID)
	if err != nil {
		t.Fatalf("effective menus for tenant %d: %v", tenantID, err)
	}
	if !sameUint32Set(got, want) {
		t.Fatalf("effective menus for tenant %d = %v, want %v", tenantID, got, want)
	}
}

func assertPackageCapabilities(t *testing.T, group *pbCore.MenuPermissionGroup, apiPermissions []string, featureFlags map[string]bool, resourceQuotas map[string]int64) {
	t.Helper()
	if !sameStringSet(group.GetApiPermissions(), apiPermissions) {
		t.Fatalf("api permissions = %v, want %v", group.GetApiPermissions(), apiPermissions)
	}
	if !reflect.DeepEqual(group.GetFeatureFlags(), featureFlags) {
		t.Fatalf("feature flags = %v, want %v", group.GetFeatureFlags(), featureFlags)
	}
	if !reflect.DeepEqual(group.GetResourceQuotas(), resourceQuotas) {
		t.Fatalf("resource quotas = %v, want %v", group.GetResourceQuotas(), resourceQuotas)
	}
}

func assertVersionCapabilities(t *testing.T, version *pbCore.MenuPermissionGroupVersion, menuIDs []uint32, apiPermissions []string, featureFlags map[string]bool, resourceQuotas map[string]int64) {
	t.Helper()
	if !sameUint32Set(version.GetMenuIds(), menuIDs) {
		t.Fatalf("version menu ids = %v, want %v", version.GetMenuIds(), menuIDs)
	}
	if !sameStringSet(version.GetApiPermissions(), apiPermissions) {
		t.Fatalf("version api permissions = %v, want %v", version.GetApiPermissions(), apiPermissions)
	}
	if !reflect.DeepEqual(version.GetFeatureFlags(), featureFlags) {
		t.Fatalf("version feature flags = %v, want %v", version.GetFeatureFlags(), featureFlags)
	}
	if !reflect.DeepEqual(version.GetResourceQuotas(), resourceQuotas) {
		t.Fatalf("version resource quotas = %v, want %v", version.GetResourceQuotas(), resourceQuotas)
	}
}
