package data

import (
	pbEnum "backend-service/api/common/enum"
	pbCore "backend-service/api/core/service/v1"
	"context"
	"io"
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

	version2, err := repo.PublishVersion(ctx, group.GetId(), []uint32{secondMenu.ID}, "replace menu", 7, "")
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
