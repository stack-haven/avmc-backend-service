package audit

import (
	"context"

	"backend-service/pkg/auth/authn"
)

// AuthnExtractor creates a ContextExtractor backed by the authn package.
func AuthnExtractor(ctx context.Context) UserInfo {
	var info UserInfo
	info.TenantID = authn.GetAuthUserTenantID(ctx)
	info.UserID = authn.GetAuthUserID(ctx)
	if user, ok := authn.AuthUserFromContext(ctx); ok && user != nil {
		info.UserName = user.Name()
	}
	return info
}
