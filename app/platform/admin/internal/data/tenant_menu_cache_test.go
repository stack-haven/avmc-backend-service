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
	if allowed, ok := repo.getTenantRoleAuthorizationCache(context.Background(), 7, 11, "object", "GET"); ok || allowed {
		t.Fatalf("authorization cache returned data while redis is unavailable: allowed=%v ok=%v", allowed, ok)
	}
	if stats := data.tenantAuthorizationCacheStatsSnapshot(); stats.Bypasses != 1 {
		t.Fatalf("authorization cache bypasses = %d, want 1", stats.Bypasses)
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

func TestTenantAuthorizationInvalidationClearsLocalSnapshotAndPersistsIntent(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()
	data := &Data{db: client}
	repo := NewBaseRepo(data, log.NewStdLogger(io.Discard))

	targetKey := tenantRoleAuthorizationCacheKey{
		tenantID:       7,
		userID:         11,
		object:         "platform.admin.v1.UserService/ListUsers",
		action:         "GET",
		menuVersion:    1,
		packageVersion: 1,
		authVersion:    1,
	}
	otherTenantKey := targetKey
	otherTenantKey.tenantID = 8
	data.authorizationCache.Store(targetKey, tenantRoleAuthorizationCacheEntry{
		allowed:   true,
		expiresAt: time.Now().Add(time.Minute),
	})
	data.authorizationCache.Store(otherTenantKey, tenantRoleAuthorizationCacheEntry{
		allowed:   true,
		expiresAt: time.Now().Add(time.Minute),
	})

	repo.bumpTenantAuthorizationVersion(context.Background(), 7)

	if _, ok := data.authorizationCache.Load(targetKey); ok {
		t.Fatal("tenant authorization cache entry was not cleared")
	}
	if _, ok := data.authorizationCache.Load(otherTenantKey); !ok {
		t.Fatal("authorization cache entry for another tenant was cleared")
	}
	if stats := data.tenantAuthorizationCacheStatsSnapshot(); stats.Clears != 1 {
		t.Fatalf("authorization cache clears = %d, want 1", stats.Clears)
	}
	task := client.AsyncTask.Query().
		Where(asynctask.TaskTypeEQ(biz.AsyncTaskTypePermissionCacheInvalidate)).
		OnlyX(systemContext())
	if task.TenantID == nil || *task.TenantID != 7 {
		t.Fatalf("authorization cache invalidation task tenant = %v, want 7", task.TenantID)
	}
	if task.Payload != `{"scope":"tenant_authorization","tenantId":7}` {
		t.Fatalf("authorization cache invalidation payload = %s", task.Payload)
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
