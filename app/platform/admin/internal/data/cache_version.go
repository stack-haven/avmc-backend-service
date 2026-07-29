package data

import "context"

// bumpMenuVersion increments the global menu version cache.
// This invalidates cached menus for all tenants when global menu structure changes.
func (r *menuRepo) bumpMenuVersion(ctx context.Context) {
	// TODO: re-implement when cache infrastructure is re-established
}

// bumpTenantPackageVersion increments the tenant package version cache.
// This invalidates cached effective menus when tenant-package bindings change.
func (r *menuPermissionGroupRepo) bumpTenantPackageVersion(ctx context.Context, tenantID uint32) {
	// TODO: re-implement when cache infrastructure is re-established
}

// bumpTenantAuthorizationVersion increments the tenant authorization cache version.
// This invalidates cached authorization decisions when user/role assignments change.
func (r *BaseRepo) bumpTenantAuthorizationVersion(ctx context.Context, tenantID uint32) {
	// TODO: re-implement when cache infrastructure is re-established
}
