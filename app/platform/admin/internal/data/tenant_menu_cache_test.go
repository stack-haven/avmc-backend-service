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
}
