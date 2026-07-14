package data

import (
	"context"
	"io"
	"testing"
	"time"

	"backend-service/app/platform/admin/internal/biz"
	"backend-service/app/platform/admin/internal/data/ent/gen/asynctask"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
)

func TestCacheVersionFailureBypassesPermissionCache(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         "127.0.0.1:1",
		MaxRetries:   -1,
		DialTimeout:  10 * time.Millisecond,
		ReadTimeout:  10 * time.Millisecond,
		WriteTimeout: 10 * time.Millisecond,
	})
	defer rdb.Close()

	client := newTestClient(t)
	defer client.Close()
	data := &Data{db: client, rdb: rdb}
	repo := NewBaseRepo(data, log.NewStdLogger(io.Discard))
	repo.bumpTenantPackageVersion(context.Background(), 7)
	if _, bypass := data.permissionCacheBypass.Load(tenantPackageVersionKey(7)); !bypass {
		t.Fatal("permission cache was not bypassed after version bump failure")
	}
	if ids, ok := repo.getTenantEffectiveMenuIDsCache(context.Background(), 7); ok || ids != nil {
		t.Fatalf("permission cache returned data while bypassed: ids=%v ok=%v", ids, ok)
	}
	task := client.AsyncTask.Query().
		Where(asynctask.TaskTypeEQ(biz.AsyncTaskTypePermissionCacheInvalidate)).
		OnlyX(systemContext())
	if task.TenantID == nil || *task.TenantID != 7 {
		t.Fatalf("cache invalidation task tenant = %v, want 7", task.TenantID)
	}
	if task.Queue != "maintenance" || task.MaxAttempts != 10 {
		t.Fatalf("cache invalidation task retry config = queue:%s attempts:%d", task.Queue, task.MaxAttempts)
	}
	if task.IdempotencyKey == nil || *task.IdempotencyKey == "" {
		t.Fatal("cache invalidation task idempotency key is empty")
	}
}

func TestPermissionCacheInvalidationIntentIsDurable(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()
	data := &Data{db: client}
	repo := NewBaseRepo(data, log.NewStdLogger(io.Discard))

	ctx := context.Background()
	repo.bumpMenuVersion(ctx)

	tasks := client.AsyncTask.Query().
		Where(asynctask.TaskTypeEQ(biz.AsyncTaskTypePermissionCacheInvalidate)).
		AllX(systemContext())
	if len(tasks) != 1 {
		t.Fatalf("cache invalidation task count = %d, want 1", len(tasks))
	}
	task := tasks[0]
	if task.TenantID != nil {
		t.Fatalf("menu cache invalidation tenant = %v, want nil", *task.TenantID)
	}
	if task.Payload != `{"scope":"menu","tenantId":0}` {
		t.Fatalf("cache invalidation payload = %s", task.Payload)
	}
	if task.IdempotencyKey == nil || *task.IdempotencyKey == "" {
		t.Fatal("cache invalidation idempotency key is empty")
	}
}

func TestPermissionCacheInvalidationIdempotencyKeyIncludesChangeTimestamp(t *testing.T) {
	base := time.Date(2026, 7, 14, 9, 8, 7, 0, time.FixedZone("CST", 8*60*60))
	got := permissionCacheInvalidationIdempotencyKey("tenant_package", 42, base)
	want := "permission-cache:tenant_package:42:1783991287000000000"
	if got != want {
		t.Fatalf("idempotency key = %s, want %s", got, want)
	}
}
