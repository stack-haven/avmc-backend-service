package biz

import (
	"context"
	"io"
	"strings"
	"testing"

	pbCore "backend-service/api/core/service/v1"
	"backend-service/pkg/auth/authn"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

type resourceQuotaRepoStub struct {
	usages map[string]*pbCore.TenantResourceQuotaUsage
}

func (r *resourceQuotaRepoStub) ListUsage(_ context.Context, tenantID uint32) ([]*pbCore.TenantResourceQuotaUsage, error) {
	items := make([]*pbCore.TenantResourceQuotaUsage, 0, len(r.usages))
	for _, usage := range r.usages {
		if usage.GetTenantId() == tenantID {
			copy := *usage
			items = append(items, &copy)
		}
	}
	return items, nil
}

func (r *resourceQuotaRepoStub) GetUsage(_ context.Context, tenantID uint32, resourceKey string) (*pbCore.TenantResourceQuotaUsage, error) {
	if r.usages == nil {
		r.usages = map[string]*pbCore.TenantResourceQuotaUsage{}
	}
	if usage := r.usages[resourceKey]; usage != nil {
		copy := *usage
		return &copy, nil
	}
	return &pbCore.TenantResourceQuotaUsage{TenantId: tenantID, ResourceKey: resourceKey}, nil
}

func (r *resourceQuotaRepoStub) Consume(_ context.Context, tenantID uint32, resourceKey string, amount int64, limit int64, unlimited bool, _ string, _ uint32) (*pbCore.TenantResourceQuotaUsage, error) {
	usage, _ := r.GetUsage(context.Background(), tenantID, resourceKey)
	if !unlimited && usage.GetUsed()+amount > limit {
		return nil, errors.Forbidden("RESOURCE_QUOTA_EXCEEDED", "资源额度不足")
	}
	usage.Used += amount
	r.usages[resourceKey] = usage
	return usage, nil
}

func (r *resourceQuotaRepoStub) Release(_ context.Context, tenantID uint32, resourceKey string, amount int64, _ string, _ uint32) (*pbCore.TenantResourceQuotaUsage, error) {
	usage, _ := r.GetUsage(context.Background(), tenantID, resourceKey)
	usage.Used -= amount
	if usage.Used < 0 {
		usage.Used = 0
	}
	r.usages[resourceKey] = usage
	return usage, nil
}

type resourceQuotaTestUser struct {
	subject string
	tenant  string
}

func (u resourceQuotaTestUser) Name() string                           { return "test" }
func (u resourceQuotaTestUser) ParseFromContext(context.Context) error { return nil }
func (u resourceQuotaTestUser) GetSubject() string                     { return u.subject }
func (u resourceQuotaTestUser) GetObject() string                      { return "" }
func (u resourceQuotaTestUser) GetAction() string                      { return "" }
func (u resourceQuotaTestUser) GetTenant() string                      { return u.tenant }

func TestResourceQuotaUsecaseConsumeCheckAndRelease(t *testing.T) {
	t.Parallel()

	ctx := authn.ContextWithAuthUser(context.Background(), resourceQuotaTestUser{subject: "7", tenant: "10"})
	uc := NewResourceQuotaUsecase(
		&resourceQuotaRepoStub{},
		&menuPermissionGroupRepoStub{caps: &pbCore.GetCurrentTenantCapabilitiesResponse{
			TenantId:       10,
			ResourceQuotas: map[string]int64{"projects": 5},
		}},
		log.NewStdLogger(io.Discard),
	)

	check, err := uc.CheckCurrent(ctx, "projects", 3)
	if err != nil {
		t.Fatalf("CheckCurrent() error = %v", err)
	}
	if !check.GetAllowed() || check.GetUsage().GetLimit() != 5 || check.GetUsage().GetRemaining() != 5 {
		t.Fatalf("check response = %v", check)
	}

	usage, err := uc.ConsumeCurrent(ctx, "projects", 3, "consume-projects-1")
	if err != nil {
		t.Fatalf("ConsumeCurrent() error = %v", err)
	}
	if usage.GetUsed() != 3 || usage.GetRemaining() != 2 {
		t.Fatalf("usage after consume = %v", usage)
	}

	if _, err = uc.ConsumeCurrent(ctx, "projects", 3, "consume-projects-2"); !errors.IsForbidden(err) {
		t.Fatalf("second consume error = %v, want quota exceeded", err)
	}

	usage, err = uc.ReleaseCurrent(ctx, "projects", 9, "release-projects-1")
	if err != nil {
		t.Fatalf("ReleaseCurrent() error = %v", err)
	}
	if usage.GetUsed() != 0 || usage.GetRemaining() != 5 {
		t.Fatalf("usage after release = %v", usage)
	}
}

func TestResourceQuotaUsecaseTreatsMissingLimitAsUnlimited(t *testing.T) {
	t.Parallel()

	ctx := authn.ContextWithAuthUser(context.Background(), resourceQuotaTestUser{subject: "7", tenant: "10"})
	uc := NewResourceQuotaUsecase(
		&resourceQuotaRepoStub{},
		&menuPermissionGroupRepoStub{caps: &pbCore.GetCurrentTenantCapabilitiesResponse{TenantId: 10}},
		log.NewStdLogger(io.Discard),
	)

	usage, err := uc.ConsumeCurrent(ctx, "custom.metric", 100, "")
	if err != nil {
		t.Fatalf("ConsumeCurrent() unlimited error = %v", err)
	}
	if !usage.GetUnlimited() || usage.GetLimit() != 0 || usage.GetUsed() != 100 {
		t.Fatalf("unlimited usage = %v", usage)
	}
}

func TestResourceQuotaUsecaseReservationRelease(t *testing.T) {
	t.Parallel()

	ctx := authn.ContextWithAuthUser(context.Background(), resourceQuotaTestUser{subject: "7", tenant: "10"})
	uc := NewResourceQuotaUsecase(
		&resourceQuotaRepoStub{},
		&menuPermissionGroupRepoStub{caps: &pbCore.GetCurrentTenantCapabilitiesResponse{
			TenantId:       10,
			ResourceQuotas: map[string]int64{"projects": 5},
		}},
		log.NewStdLogger(io.Discard),
	)

	reservation, usage, err := uc.ReserveCurrent(ctx, "projects", 2, "project-create-42")
	if err != nil {
		t.Fatalf("ReserveCurrent() error = %v", err)
	}
	if usage.GetUsed() != 2 || usage.GetRemaining() != 3 {
		t.Fatalf("usage after reserve = %v", usage)
	}

	usage, err = reservation.Release(ctx)
	if err != nil {
		t.Fatalf("reservation Release() error = %v", err)
	}
	if usage.GetUsed() != 0 || usage.GetRemaining() != 5 {
		t.Fatalf("usage after release = %v", usage)
	}
}

func TestResourceQuotaUsecaseReservationRequiresIdempotencyKey(t *testing.T) {
	t.Parallel()

	ctx := authn.ContextWithAuthUser(context.Background(), resourceQuotaTestUser{subject: "7", tenant: "10"})
	uc := NewResourceQuotaUsecase(
		&resourceQuotaRepoStub{},
		&menuPermissionGroupRepoStub{caps: &pbCore.GetCurrentTenantCapabilitiesResponse{
			TenantId:       10,
			ResourceQuotas: map[string]int64{"projects": 5},
		}},
		log.NewStdLogger(io.Discard),
	)

	if _, _, err := uc.ReserveCurrent(ctx, "projects", 1, ""); !errors.IsBadRequest(err) {
		t.Fatalf("ReserveCurrent() error = %v, want bad request", err)
	}
}

func TestResourceQuotaUsecaseRequiresTenantContext(t *testing.T) {
	t.Parallel()

	uc := NewResourceQuotaUsecase(&resourceQuotaRepoStub{}, &menuPermissionGroupRepoStub{}, log.NewStdLogger(io.Discard))
	if _, err := uc.CheckCurrent(context.Background(), "projects", 1); !errors.IsForbidden(err) {
		t.Fatalf("missing tenant error = %v, want forbidden", err)
	}
}

func TestResourceQuotaUsecaseRejectsInvalidIdempotencyKey(t *testing.T) {
	t.Parallel()

	ctx := authn.ContextWithAuthUser(context.Background(), resourceQuotaTestUser{subject: "7", tenant: "10"})
	uc := NewResourceQuotaUsecase(
		&resourceQuotaRepoStub{},
		&menuPermissionGroupRepoStub{caps: &pbCore.GetCurrentTenantCapabilitiesResponse{
			TenantId:       10,
			ResourceQuotas: map[string]int64{"projects": 5},
		}},
		log.NewStdLogger(io.Discard),
	)

	if _, err := uc.ConsumeCurrent(ctx, "projects", 1, strings.Repeat("x", 121)); !errors.IsBadRequest(err) {
		t.Fatalf("ConsumeCurrent() error = %v, want bad request", err)
	}
}
