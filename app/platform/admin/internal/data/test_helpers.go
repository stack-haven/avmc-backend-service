package data

import (
	"context"

	entviewer "backend-service/app/platform/admin/internal/data/ent/viewer"
)

// tenantContext returns a context with the given tenant ID set for testing.
func tenantContext(tenantID uint32) context.Context {
	return entviewer.NewTenantContext(context.Background(), tenantID)
}

// tenantUserContext returns a context with the given tenant ID and user ID for testing.
func tenantUserContext(tenantID, userID uint32) context.Context {
	return entviewer.NewTenantContext(context.Background(), tenantID)
}

// systemContext returns a context that bypasses tenant privacy filters for testing.
func systemContext() context.Context {
	return entviewer.NewSystemContext(context.Background())
}

