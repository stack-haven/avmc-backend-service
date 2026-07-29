package data

import (
	"context"

	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/platform/admin/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

type resourceQuotaRepoStub struct {
	log *log.Helper
}

// NewResourceQuotaRepo creates a stub resource quota repository.
func NewResourceQuotaRepo(logger log.Logger) biz.ResourceQuotaRepo {
	return &resourceQuotaRepoStub{log: log.NewHelper(logger)}
}

func (r *resourceQuotaRepoStub) ListUsage(_ context.Context, _ uint32) ([]*pbCore.TenantResourceQuotaUsage, error) {
	return nil, nil
}

func (r *resourceQuotaRepoStub) GetUsage(_ context.Context, _ uint32, _ string) (*pbCore.TenantResourceQuotaUsage, error) {
	return nil, nil
}

func (r *resourceQuotaRepoStub) Consume(_ context.Context, _ uint32, _ string, _, _ int64, _ bool, _ string, _ uint32) (*pbCore.TenantResourceQuotaUsage, bool, error) {
	return nil, false, nil
}

func (r *resourceQuotaRepoStub) Release(_ context.Context, _ uint32, _ string, _ int64, _ string, _ uint32) (*pbCore.TenantResourceQuotaUsage, error) {
	return nil, nil
}
