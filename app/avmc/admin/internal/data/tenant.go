package data

import (
	"context"

	"backend-service/pkg/auth/authn"

	"github.com/go-kratos/kratos/v2/errors"
)

func requireDomainID(ctx context.Context) (uint32, error) {
	domainID := authn.GetAuthUserDomainID(ctx)
	if domainID == 0 {
		return 0, errors.Forbidden("TENANT_CONTEXT_REQUIRED", "缺少有效的数据域上下文")
	}
	return domainID, nil
}
