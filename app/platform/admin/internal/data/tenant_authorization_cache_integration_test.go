//go:build integration

package data

import (
	"context"
	"io"
	"os"
	"testing"
	"time"

	"backend-service/app/platform/admin/internal/runtimeconfig"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
)

func TestTenantAuthorizationCacheWithRedisVersions(t *testing.T) {
	ctx := context.Background()
	cfg, cfgErr := runtimeconfig.Load("../../configs")
	addr := os.Getenv("AVMC_TEST_REDIS_ADDR")
	if addr == "" {
		addr = os.Getenv("platform_ADMIN_REDIS_ADDR")
	}
	if addr == "" && cfgErr == nil && cfg.Data != nil && cfg.Data.Redis != nil {
		addr = cfg.Data.Redis.GetAddr()
	}
	if addr == "" {
		addr = "127.0.0.1:6379"
	}
	password := os.Getenv("AVMC_TEST_REDIS_PASSWORD")
	if password == "" {
		password = os.Getenv("platform_ADMIN_REDIS_PASSWORD")
	}
	if password == "" && cfgErr == nil && cfg.Data != nil && cfg.Data.Redis != nil {
		password = cfg.Data.Redis.GetPassword()
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           0,
		DialTimeout:  500 * time.Millisecond,
		ReadTimeout:  500 * time.Millisecond,
		WriteTimeout: 500 * time.Millisecond,
	})
	defer rdb.Close()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("redis is unavailable at %s: %v", addr, err)
	}

	tenantID := uint32(991)
	userID := uint32(1991)
	object := "platform.admin.v1.UserService/ListUsers"
	action := "GET"
	keys := []string{menuVersionKey(), tenantPackageVersionKey(tenantID), tenantAuthorizationVersionKey(tenantID)}
	t.Cleanup(func() {
		_ = rdb.Del(context.Background(), keys...).Err()
	})
	for _, key := range keys {
		if err := rdb.Set(ctx, key, 1, time.Minute).Err(); err != nil {
			t.Fatalf("set redis version %s: %v", key, err)
		}
	}

	data := &Data{rdb: rdb}
	repo := NewBaseRepo(data, log.NewStdLogger(io.Discard))
	repo.setTenantRoleAuthorizationCache(ctx, tenantID, userID, object, action, true)
	allowed, ok := repo.getTenantRoleAuthorizationCache(ctx, tenantID, userID, object, action)
	if !ok || !allowed {
		t.Fatalf("authorization cache read = allowed:%v ok:%v, want true/true", allowed, ok)
	}

	invalidator := NewPermissionCacheInvalidator(data)
	if err := invalidator.InvalidateTenantAuthorizationCache(ctx, tenantID); err != nil {
		t.Fatalf("invalidate tenant authorization cache: %v", err)
	}
	allowed, ok = repo.getTenantRoleAuthorizationCache(ctx, tenantID, userID, object, action)
	if ok || allowed {
		t.Fatalf("authorization cache survived version invalidation: allowed:%v ok:%v", allowed, ok)
	}
	stats := data.tenantAuthorizationCacheStatsSnapshot()
	if stats.Sets != 1 || stats.Hits != 1 || stats.Misses != 1 || stats.Invalidations != 1 || stats.Clears != 1 {
		t.Fatalf("authorization cache stats = %+v, want one set/hit/miss/invalidation/clear", stats)
	}
}
