package data

import (
	"io"
	"testing"

	pbEnum "backend-service/api/common/enum"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

func TestResourceQuotaRepoConsumeAndRelease(t *testing.T) {
	ctx := systemContext()
	client := newTestClient(t)
	defer client.Close()

	tenant := client.Tenant.Create().
		SetName("Quota Tenant").
		SetCode("quota-tenant").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		SaveX(ctx)
	repo := NewResourceQuotaRepo(&Data{db: client}, log.NewStdLogger(io.Discard))

	usage, err := repo.Consume(ctx, tenant.ID, "projects", 3, 5, false, 7)
	if err != nil {
		t.Fatalf("consume quota: %v", err)
	}
	if usage.GetUsed() != 3 {
		t.Fatalf("usage after consume = %v, want 3", usage.GetUsed())
	}
	if _, err = repo.Consume(ctx, tenant.ID, "projects", 3, 5, false, 7); !errors.IsForbidden(err) {
		t.Fatalf("consume over quota error = %v, want quota exceeded", err)
	}

	usage, err = repo.Release(ctx, tenant.ID, "projects", 10, 7)
	if err != nil {
		t.Fatalf("release quota: %v", err)
	}
	if usage.GetUsed() != 0 {
		t.Fatalf("usage after release = %v, want 0", usage.GetUsed())
	}

	items, err := repo.ListUsage(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("list quota usage: %v", err)
	}
	if len(items) != 1 || items[0].GetResourceKey() != "projects" {
		t.Fatalf("usage items = %v", items)
	}
}

func TestResourceQuotaRepoUnlimitedUsage(t *testing.T) {
	ctx := systemContext()
	client := newTestClient(t)
	defer client.Close()

	tenant := client.Tenant.Create().
		SetName("Unlimited Quota Tenant").
		SetCode("unlimited-quota-tenant").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		SaveX(ctx)
	repo := NewResourceQuotaRepo(&Data{db: client}, log.NewStdLogger(io.Discard))

	usage, err := repo.Consume(ctx, tenant.ID, "custom.metric", 100, 0, true, 0)
	if err != nil {
		t.Fatalf("consume unlimited quota: %v", err)
	}
	if usage.GetUsed() != 100 {
		t.Fatalf("unlimited usage = %v, want 100", usage.GetUsed())
	}
}
