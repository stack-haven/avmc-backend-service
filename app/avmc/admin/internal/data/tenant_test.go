package data

import (
	"context"
	"strconv"
	"testing"

	"backend-service/pkg/auth/authn"
)

type testSecurityUser struct {
	subject string
	domain  string
}

func (u testSecurityUser) Name() string                           { return "test" }
func (u testSecurityUser) ParseFromContext(context.Context) error { return nil }
func (u testSecurityUser) GetSubject() string                     { return u.subject }
func (u testSecurityUser) GetObject() string                      { return "" }
func (u testSecurityUser) GetAction() string                      { return "" }
func (u testSecurityUser) GetDomain() string                      { return u.domain }

func tenantContext(domainID uint32) context.Context {
	return authn.ContextWithAuthUser(context.Background(), testSecurityUser{
		subject: "1",
		domain:  strconv.FormatUint(uint64(domainID), 10),
	})
}

func TestRequireDomainID(t *testing.T) {
	t.Parallel()

	if _, err := requireDomainID(context.Background()); err == nil {
		t.Fatal("requireDomainID() error = nil without authenticated domain")
	}
	if got, err := requireDomainID(tenantContext(7)); err != nil || got != 7 {
		t.Fatalf("requireDomainID() = %d, %v; want 7, nil", got, err)
	}
}
