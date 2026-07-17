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
	data := &Data{db: client}
	repo := NewResourceQuotaRepo(data, log.NewStdLogger(io.Discard))

	usage, err := repo.Consume(ctx, tenant.ID, "projects", 3, 5, false, "", 7)
	if err != nil {
		t.Fatalf("consume quota: %v", err)
	}
	if usage.GetUsed() != 3 {
		t.Fatalf("usage after consume = %v, want 3", usage.GetUsed())
	}
	if _, err = repo.Consume(ctx, tenant.ID, "projects", 3, 5, false, "", 7); !errors.IsForbidden(err) {
		t.Fatalf("consume over quota error = %v, want quota exceeded", err)
	}

	usage, err = repo.Release(ctx, tenant.ID, "projects", 10, "", 7)
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
	stats := data.resourceQuotaStatsSnapshot()
	if stats.Consumes != 2 || stats.Releases != 1 || stats.QuotaExceeded != 1 {
		t.Fatalf("quota stats = %+v", stats)
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
	data := &Data{db: client}
	repo := NewResourceQuotaRepo(data, log.NewStdLogger(io.Discard))

	usage, err := repo.Consume(ctx, tenant.ID, "custom.metric", 100, 0, true, "", 0)
	if err != nil {
		t.Fatalf("consume unlimited quota: %v", err)
	}
	if usage.GetUsed() != 100 {
		t.Fatalf("unlimited usage = %v, want 100", usage.GetUsed())
	}
}

func TestResourceQuotaRepoConsumeIsIdempotent(t *testing.T) {
	ctx := systemContext()
	client := newTestClient(t)
	defer client.Close()

	tenant := client.Tenant.Create().
		SetName("Idempotent Consume Tenant").
		SetCode("idempotent-consume-tenant").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		SaveX(ctx)
	data := &Data{db: client}
	repo := NewResourceQuotaRepo(data, log.NewStdLogger(io.Discard))

	usage, err := repo.Consume(ctx, tenant.ID, "projects", 3, 10, false, "consume-key-1", 7)
	if err != nil {
		t.Fatalf("consume quota: %v", err)
	}
	if usage.GetUsed() != 3 {
		t.Fatalf("usage after first consume = %v, want 3", usage.GetUsed())
	}

	usage, err = repo.Consume(ctx, tenant.ID, "projects", 3, 10, false, "consume-key-1", 7)
	if err != nil {
		t.Fatalf("replay consume quota: %v", err)
	}
	if usage.GetUsed() != 3 {
		t.Fatalf("usage after replay consume = %v, want 3", usage.GetUsed())
	}

	if _, err = repo.Consume(ctx, tenant.ID, "projects", 4, 10, false, "consume-key-1", 7); !errors.IsConflict(err) {
		t.Fatalf("consume idempotency conflict = %v, want conflict", err)
	}
	stats := data.resourceQuotaStatsSnapshot()
	if stats.Consumes != 3 || stats.IdempotencyConflicts != 1 {
		t.Fatalf("quota stats = %+v", stats)
	}
}

func TestResourceQuotaRepoReleaseIsIdempotent(t *testing.T) {
	ctx := systemContext()
	client := newTestClient(t)
	defer client.Close()

	tenant := client.Tenant.Create().
		SetName("Idempotent Release Tenant").
		SetCode("idempotent-release-tenant").
		SetStatus(int32(pbEnum.Status_STATUS_ENABLED)).
		SaveX(ctx)
	repo := NewResourceQuotaRepo(&Data{db: client}, log.NewStdLogger(io.Discard))

	if _, err := repo.Consume(ctx, tenant.ID, "projects", 6, 10, false, "consume-before-release", 7); err != nil {
		t.Fatalf("consume quota: %v", err)
	}
	usage, err := repo.Release(ctx, tenant.ID, "projects", 2, "release-key-1", 7)
	if err != nil {
		t.Fatalf("release quota: %v", err)
	}
	if usage.GetUsed() != 4 {
		t.Fatalf("usage after first release = %v, want 4", usage.GetUsed())
	}

	usage, err = repo.Release(ctx, tenant.ID, "projects", 2, "release-key-1", 7)
	if err != nil {
		t.Fatalf("replay release quota: %v", err)
	}
	if usage.GetUsed() != 4 {
		t.Fatalf("usage after replay release = %v, want 4", usage.GetUsed())
	}

	if _, err = repo.Release(ctx, tenant.ID, "projects", 3, "release-key-1", 7); !errors.IsConflict(err) {
		t.Fatalf("release idempotency conflict = %v, want conflict", err)
	}
}
