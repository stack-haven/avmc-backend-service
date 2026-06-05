package data

import (
	"context"
	"strconv"
	"testing"

	"backend-service/pkg/auth/authn"
)

type testSecurityUser struct {
	subject string
	tenant  string
}

func (u testSecurityUser) Name() string                           { return "test" }
func (u testSecurityUser) ParseFromContext(context.Context) error { return nil }
func (u testSecurityUser) GetSubject() string                     { return u.subject }
func (u testSecurityUser) GetObject() string                      { return "" }
func (u testSecurityUser) GetAction() string                      { return "" }
func (u testSecurityUser) GetTenant() string                      { return u.tenant }

func tenantContext(tenantID uint32) context.Context {
	return authn.ContextWithAuthUser(context.Background(), testSecurityUser{
		subject: "1",
		tenant:  strconv.FormatUint(uint64(tenantID), 10),
	})
}

func TestRequireTenantID(t *testing.T) {
	t.Parallel()

	if _, err := requireTenantID(context.Background()); err == nil {
		t.Fatal("requireTenantID() error = nil without authenticated tenant")
	}
	if got, err := requireTenantID(tenantContext(7)); err != nil || got != 7 {
		t.Fatalf("requireTenantID() = %d, %v; want 7, nil", got, err)
	}
}
