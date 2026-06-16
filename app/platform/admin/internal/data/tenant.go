package data

import (
	"context"

	"backend-service/pkg/auth/authn"

	"github.com/go-kratos/kratos/v2/errors"
)

func requireTenantID(ctx context.Context) (uint32, error) {
	tenantID := authn.GetAuthUserTenantID(ctx)
	if tenantID == 0 {
		return 0, errors.Forbidden("TENANT_CONTEXT_REQUIRED", "缺少有效的数据租户上下文")
	}
	return tenantID, nil
}
