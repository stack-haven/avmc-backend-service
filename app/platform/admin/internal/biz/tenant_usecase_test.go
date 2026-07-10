package biz

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	pbEnum "backend-service/api/common/enum"
	pbCore "backend-service/api/core/service/v1"
	"backend-service/pkg/aip/listing"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/protobuf/proto"
)

type stubTenantRepo struct {
	current     *pbCore.Tenant
	provision   *TenantProvisioningResult
	provisioned *TenantProvisioning
	updated     *pbCore.Tenant
	deletedID   uint32
	lifecycleID uint32
	lifecycle   pbCore.TenantLifecycleStatus
	rolledBack  *TenantProvisioningResult

	provisionErr error
	rollbackErr  error
	updateErr    error
	findErr      error
	deleteErr    error
	lifecycleErr error
}

func (r *stubTenantRepo) Save(context.Context, *pbCore.Tenant, uint32) (*pbCore.Tenant, error) {
	return nil, nil
}

func (r *stubTenantRepo) Provision(_ context.Context, input *TenantProvisioning) (*TenantProvisioningResult, error) {
	if r.provisionErr != nil {
		return nil, r.provisionErr
	}
	r.provisioned = input
	if r.provision != nil {
		return r.provision, nil
	}
	return &TenantProvisioningResult{Tenant: input.Tenant, AdminUserID: 100}, nil
}

func (r *stubTenantRepo) RollbackProvisioning(_ context.Context, result *TenantProvisioningResult) error {
	r.rolledBack = result
	return r.rollbackErr
}

func (r *stubTenantRepo) Update(_ context.Context, tenant *pbCore.Tenant, _ uint32) (*pbCore.Tenant, error) {
	if r.updateErr != nil {
		return nil, r.updateErr
	}
	r.updated = tenant
	return tenant, nil
}

func (r *stubTenantRepo) FindByID(context.Context, uint32) (*pbCore.Tenant, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	if r.current == nil {
		return &pbCore.Tenant{}, nil
	}
	return proto.Clone(r.current).(*pbCore.Tenant), nil
}

func (*stubTenantRepo) CountTenants(context.Context, ...listing.Option) (int32, error) { return 0, nil }
func (*stubTenantRepo) ListTenants(context.Context, ...listing.Option) ([]*pbCore.Tenant, error) {
	return nil, nil
}

func (r *stubTenantRepo) Delete(context.Context, uint32) error {
	if r.deleteErr != nil {
		return r.deleteErr
	}
	r.deletedID = 1
	return nil
}

func (*stubTenantRepo) UpdateStatus(context.Context, uint32, pbEnum.Status) (*pbCore.Tenant, error) {
	return nil, nil
}

func (r *stubTenantRepo) UpdateLifecycle(_ context.Context, id uint32, status pbCore.TenantLifecycleStatus) (*pbCore.Tenant, error) {
	if r.lifecycleErr != nil {
		return nil, r.lifecycleErr
	}
	r.lifecycleID = id
	r.lifecycle = status
	tenant := &pbCore.Tenant{Id: id, LifecycleStatus: &status}
	return tenant, nil
}

type tenantAdminPolicyStub struct {
	calls []tenantAdminPolicyCall
	err   error
}

type tenantAdminPolicyCall struct {
	tenantID uint32
	userID   uint32
	enabled  bool
}

func (p *tenantAdminPolicyStub) SetMembership(_ context.Context, tenantID, userID uint32, enabled bool) error {
	p.calls = append(p.calls, tenantAdminPolicyCall{tenantID: tenantID, userID: userID, enabled: enabled})
	if enabled && p.err != nil {
		return p.err
	}
	return nil
}

func TestTenantUsecaseCreateValidation(t *testing.T) {
	t.Parallel()

	validTenant := func() *pbCore.Tenant {
		name := "测试租户"
		code := "tenant_demo"
		return &pbCore.Tenant{Name: &name, Code: &code, GroupIds: []uint32{1}}
	}
	validAdmin := func() *pbCore.TenantInitialAdmin {
		email := "admin@example.com"
		return &pbCore.TenantInitialAdmin{
			Username: "tenant_admin",
			Password: "Str0ng!Tenant#2026",
			Email:    &email,
		}
	}
	invalidExpiry := "2026/08/01"

	tests := []struct {
		name   string
		tenant *pbCore.Tenant
		admin  *pbCore.TenantInitialAdmin
	}{
		{name: "missing tenant", admin: validAdmin()},
		{name: "missing admin", tenant: validTenant()},
		{name: "missing group", tenant: &pbCore.Tenant{}, admin: validAdmin()},
		{name: "invalid admin username", tenant: validTenant(), admin: &pbCore.TenantInitialAdmin{Username: "ad", Password: "Str0ng!Tenant#2026"}},
		{name: "invalid admin email", tenant: validTenant(), admin: &pbCore.TenantInitialAdmin{Username: "tenant_admin", Password: "Str0ng!Tenant#2026", Email: stringPtr("bad email")}},
		{name: "weak admin password", tenant: validTenant(), admin: &pbCore.TenantInitialAdmin{Username: "tenant_admin", Password: "weakpass"}},
		{name: "invalid expiry format", tenant: &pbCore.Tenant{GroupIds: []uint32{1}, ExpiresAt: &invalidExpiry}, admin: validAdmin()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &stubTenantRepo{}
			uc := NewTenantUsecase(repo, nil, nil, log.NewStdLogger(io.Discard))
			_, err := uc.Create(context.Background(), tt.tenant, tt.admin, 7)
			if !kratoserrors.IsBadRequest(err) {
				t.Fatalf("Create() error = %v, want bad request", err)
			}
			if repo.provisioned != nil {
				t.Fatal("invalid tenant was provisioned")
			}
		})
	}
}

func TestTenantUsecaseCreateHashesPasswordAndSetsAdminPolicy(t *testing.T) {
	t.Parallel()

	name := "平台租户"
	code := "platform_tenant"
	tenant := &pbCore.Tenant{Id: 10, Name: &name, Code: &code, GroupIds: []uint32{2}}
	admin := &pbCore.TenantInitialAdmin{Username: "tenant_admin", Password: "Str0ng!Tenant#2026"}
	repo := &stubTenantRepo{provision: &TenantProvisioningResult{Tenant: tenant, AdminUserID: 99}}
	policy := &tenantAdminPolicyStub{}
	uc := NewTenantUsecase(repo, nil, policy, log.NewStdLogger(io.Discard))

	result, err := uc.Create(context.Background(), tenant, admin, 7)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if result.AdminUserID != 99 {
		t.Fatalf("admin user id = %d, want 99", result.AdminUserID)
	}
	if repo.provisioned == nil || repo.provisioned.OperatorID != 7 || repo.provisioned.AdminUsername != "tenant_admin" {
		t.Fatalf("provisioning input = %#v", repo.provisioned)
	}
	if repo.provisioned.AdminPasswordHash == admin.Password || !strings.HasPrefix(repo.provisioned.AdminPasswordHash, "$2") {
		t.Fatalf("admin password was not bcrypt hashed: %q", repo.provisioned.AdminPasswordHash)
	}
	if len(policy.calls) != 1 || policy.calls[0] != (tenantAdminPolicyCall{tenantID: 10, userID: 99, enabled: true}) {
		t.Fatalf("policy calls = %#v", policy.calls)
	}
}

func TestTenantUsecaseCreateRollsBackProvisioningWhenPolicyFails(t *testing.T) {
	t.Parallel()

	name := "平台租户"
	tenant := &pbCore.Tenant{Id: 10, Name: &name, GroupIds: []uint32{2}}
	expected := errors.New("policy failed")
	result := &TenantProvisioningResult{Tenant: tenant, AdminUserID: 99}
	repo := &stubTenantRepo{provision: result, rollbackErr: errors.New("rollback failed")}
	policy := &tenantAdminPolicyStub{err: expected}
	uc := NewTenantUsecase(repo, nil, policy, log.NewStdLogger(io.Discard))

	_, err := uc.Create(context.Background(), tenant, &pbCore.TenantInitialAdmin{Username: "tenant_admin", Password: "Str0ng!Tenant#2026"}, 7)
	if !errors.Is(err, expected) {
		t.Fatalf("Create() error = %v, want policy error", err)
	}
	if repo.rolledBack != result {
		t.Fatalf("rollback result = %#v, want %#v", repo.rolledBack, result)
	}
	if len(policy.calls) != 2 || policy.calls[0].enabled != true || policy.calls[1].enabled != false {
		t.Fatalf("policy cleanup calls = %#v", policy.calls)
	}
}

func TestTenantUsecaseUpdateRevokesSessionsWhenExpiryChanges(t *testing.T) {
	t.Parallel()

	oldExpiry := "2026-08-01 00:00:00"
	newExpiry := "2026-09-01 00:00:00"
	repo := &stubTenantRepo{current: &pbCore.Tenant{Id: 12, ExpiresAt: &oldExpiry}}
	sessions := &stubSessionRepo{}
	uc := NewTenantUsecase(repo, sessions, nil, log.NewStdLogger(io.Discard))

	if _, err := uc.Update(context.Background(), &pbCore.Tenant{Id: 12, ExpiresAt: &newExpiry}, 7); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if sessions.revokedTenant != 12 {
		t.Fatalf("revoked tenant = %d, want 12", sessions.revokedTenant)
	}
}

func TestTenantUsecaseDeleteRevokesTenantSessions(t *testing.T) {
	t.Parallel()

	repo := &stubTenantRepo{current: &pbCore.Tenant{Id: 12}}
	sessions := &stubSessionRepo{}
	uc := NewTenantUsecase(repo, sessions, nil, log.NewStdLogger(io.Discard))

	if err := uc.Delete(context.Background(), 12); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if sessions.revokedTenant != 12 {
		t.Fatalf("revoked tenant = %d, want 12", sessions.revokedTenant)
	}
}

func TestTenantUsecaseUpdateStatusMapsToLifecycle(t *testing.T) {
	t.Parallel()

	active := pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_ACTIVE
	repo := &stubTenantRepo{current: &pbCore.Tenant{Id: 12, LifecycleStatus: &active}}
	sessions := &stubSessionRepo{}
	uc := NewTenantUsecase(repo, sessions, nil, log.NewStdLogger(io.Discard))

	if _, err := uc.UpdateStatus(context.Background(), 12, pbEnum.Status_STATUS_DISABLED); err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	if repo.lifecycle != pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_SUSPENDED {
		t.Fatalf("lifecycle = %v, want suspended", repo.lifecycle)
	}
	if sessions.revokedTenant != 12 {
		t.Fatalf("revoked tenant = %d, want 12", sessions.revokedTenant)
	}
}

func TestTenantUsecaseUpdateLifecycleRejectsInvalidTransition(t *testing.T) {
	t.Parallel()

	cancelled := pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_CANCELLED
	repo := &stubTenantRepo{current: &pbCore.Tenant{Id: 12, LifecycleStatus: &cancelled}}
	uc := NewTenantUsecase(repo, nil, nil, log.NewStdLogger(io.Discard))

	_, err := uc.UpdateLifecycle(context.Background(), 12, pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_ACTIVE)
	if !kratoserrors.IsBadRequest(err) {
		t.Fatalf("UpdateLifecycle() error = %v, want bad request", err)
	}
	if repo.lifecycleID != 0 {
		t.Fatalf("invalid lifecycle transition was persisted for tenant %d", repo.lifecycleID)
	}
}
