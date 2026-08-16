package data

import (
	"context"

	"backend-service/app/platform/service/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

type permissionCacheInvalidatorStub struct {
	log *log.Helper
}

// NewPermissionCacheInvalidator creates a stub cache invalidator.
func NewPermissionCacheInvalidator(logger log.Logger) biz.PermissionCacheInvalidator {
	return &permissionCacheInvalidatorStub{log: log.NewHelper(logger)}
}

func (s *permissionCacheInvalidatorStub) InvalidateMenuPermissionCache(_ context.Context) error {
	return nil
}

func (s *permissionCacheInvalidatorStub) InvalidateTenantPackagePermissionCache(_ context.Context, _ uint32) error {
	return nil
}

func (s *permissionCacheInvalidatorStub) InvalidateTenantAuthorizationCache(_ context.Context, _ uint32) error {
	return nil
}
