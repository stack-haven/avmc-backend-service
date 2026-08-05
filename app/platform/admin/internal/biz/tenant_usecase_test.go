package biz

import (
	"context"
	"testing"

	pbCore "backend-service/api/core/service/v1"
)

func TestTenantLifecycleTransitionAllowed(t *testing.T) {
	t.Parallel()

	type transition struct {
		current pbCore.TenantLifecycleStatus
		target  pbCore.TenantLifecycleStatus
		allowed bool
	}

	tests := []struct {
		name        string
		transitions []transition
	}{
		{
			name: "PENDING transitions",
			transitions: []transition{
				{current: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_PENDING, target: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_ACTIVE, allowed: true},
				{current: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_PENDING, target: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_CANCELLED, allowed: true},
				{current: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_PENDING, target: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_SUSPENDED, allowed: false},
				{current: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_PENDING, target: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_EXPIRED, allowed: false},
			},
		},
		{
			name: "ACTIVE transitions",
			transitions: []transition{
				{current: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_ACTIVE, target: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_SUSPENDED, allowed: true},
				{current: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_ACTIVE, target: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_EXPIRED, allowed: true},
				{current: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_ACTIVE, target: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_CANCELLED, allowed: true},
				{current: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_ACTIVE, target: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_PENDING, allowed: false},
			},
		},
		{
			name: "SUSPENDED transitions",
			transitions: []transition{
				{current: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_SUSPENDED, target: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_ACTIVE, allowed: true},
				{current: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_SUSPENDED, target: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_EXPIRED, allowed: true},
				{current: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_SUSPENDED, target: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_CANCELLED, allowed: true},
				{current: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_SUSPENDED, target: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_PENDING, allowed: false},
			},
		},
		{
			name: "EXPIRED transitions",
			transitions: []transition{
				{current: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_EXPIRED, target: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_ACTIVE, allowed: true},
				{current: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_EXPIRED, target: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_CANCELLED, allowed: true},
				{current: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_EXPIRED, target: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_PENDING, allowed: false},
				{current: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_EXPIRED, target: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_SUSPENDED, allowed: false},
			},
		},
		{
			name: "CANCELLED is terminal",
			transitions: []transition{
				{current: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_CANCELLED, target: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_ACTIVE, allowed: false},
				{current: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_CANCELLED, target: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_PENDING, allowed: false},
				{current: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_CANCELLED, target: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_SUSPENDED, allowed: false},
				{current: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_CANCELLED, target: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_EXPIRED, allowed: false},
			},
		},
		{
			name: "self-transition is always allowed",
			transitions: []transition{
				{current: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_ACTIVE, target: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_ACTIVE, allowed: true},
				{current: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_CANCELLED, target: pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_CANCELLED, allowed: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, tr := range tt.transitions {
				got := tenantLifecycleTransitionAllowed(tr.current, tr.target)
				if got != tr.allowed {
					t.Errorf("transition %v → %v: got %v, want %v",
						tr.current, tr.target, got, tr.allowed)
				}
			}
		})
	}
}

// TenantRepoStub is a minimal stub for unit-testing TenantUsecase methods that
// does not require a database. Add more methods as needed.
type TenantRepoStub struct {
	findByIDResult *pbCore.Tenant
	findByIDErr    error
}

func (r *TenantRepoStub) Provision(_ context.Context, _ *TenantProvisioning) (*TenantProvisioningResult, error) {
	return nil, nil
}
func (r *TenantRepoStub) RollbackProvisioning(_ context.Context, _ *TenantProvisioningResult) error {
	return nil
}
func (r *TenantRepoStub) Update(_ context.Context, _ *pbCore.Tenant, _ uint32) (*pbCore.Tenant, error) {
	return nil, nil
}
func (r *TenantRepoStub) FindByID(_ context.Context, _ uint32) (*pbCore.Tenant, error) {
	return r.findByIDResult, r.findByIDErr
}
func (r *TenantRepoStub) CountTenants(_ context.Context, _ ...interface{}) (int32, error) {
	return 0, nil
}
func (r *TenantRepoStub) ListTenants(_ context.Context, _ ...interface{}) ([]*pbCore.Tenant, error) {
	return nil, nil
}
func (r *TenantRepoStub) Delete(_ context.Context, _ uint32) error          { return nil }
func (r *TenantRepoStub) UpdateLifecycle(_ context.Context, _ uint32, _ pbCore.TenantLifecycleStatus) (*pbCore.Tenant, error) {
	return nil, nil
}
func (r *TenantRepoStub) ListAdmins(_ context.Context, _ uint32) ([]*pbCore.User, error) {
	return nil, nil
}
func (r *TenantRepoStub) UpdateAdmin(_ context.Context, _ uint32, _ uint32, _, _, _ *string) (*pbCore.User, error) {
	return nil, nil
}
func (r *TenantRepoStub) ResetAdminPassword(_ context.Context, _, _ uint32, _ string) error {
	return nil
}
