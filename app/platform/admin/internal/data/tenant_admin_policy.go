package data

import (
	"context"
	"strconv"

	"backend-service/app/platform/admin/internal/authzpolicy"
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/app/platform/admin/internal/data/ent/gen"
	"backend-service/app/platform/admin/internal/data/ent/gen/tenant"
	entviewer "backend-service/app/platform/admin/internal/data/ent/viewer"
	"backend-service/pkg/auth/authz"
)

type tenantAdminPolicy struct {
	data       *Data
	authorizer authz.Authorizer
}

func NewTenantAdminPolicy(data *Data, authorizer authz.Authorizer) biz.TenantAdminPolicy {
	return &tenantAdminPolicy{data: data, authorizer: authorizer}
}

func (p *tenantAdminPolicy) SetMembership(ctx context.Context, tenantID, userID uint32, enabled bool) error {
	systemCtx := entviewer.NewSystemContext(ctx)
	item, err := p.data.DB(systemCtx).Tenant.Query().
		Where(tenant.IDEQ(tenantID)).
		Select(tenant.FieldIsPlatform).
		Only(systemCtx)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil
		}
		return err
	}
	return authzpolicy.SetAdminMembership(
		ctx,
		p.authorizer,
		authz.Tenant(strconv.FormatUint(uint64(tenantID), 10)),
		authz.Subject(strconv.FormatUint(uint64(userID), 10)),
		item.IsPlatform,
		enabled,
	)
}
