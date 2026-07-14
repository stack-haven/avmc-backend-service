package data

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	pb "backend-service/api/core/service/v1"
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/app/platform/admin/internal/data/ent/gen"
	"backend-service/app/platform/admin/internal/data/ent/gen/asynctask"
	entviewer "backend-service/app/platform/admin/internal/data/ent/viewer"

	"github.com/redis/go-redis/v9"
)

const tenantEffectiveMenuCacheTTL = 30 * time.Minute

func tenantPackageVersionKey(tenantID uint32) string {
	return fmt.Sprintf("platform:admin:tenant:%d:package_version", tenantID)
}

func menuVersionKey() string {
	return "platform:admin:menu:version"
}

func tenantEffectiveMenuIDsCacheKey(tenantID uint32, menuVersion, packageVersion uint64) string {
	return fmt.Sprintf("platform:admin:tenant:%d:effective_menu_ids:%d:%d", tenantID, menuVersion, packageVersion)
}

func (r *BaseRepo) cacheUint64(ctx context.Context, key string) (uint64, bool) {
	if r == nil || r.Data == nil || r.Data.rdb == nil {
		return 0, false
	}
	value, err := r.Data.rdb.Get(ctx, key).Uint64()
	if err == redis.Nil {
		return 0, true
	}
	if err != nil {
		r.Log.WithContext(ctx).Warnf("读取缓存版本失败 key=%s err=%v", key, err)
		return 0, false
	}
	return value, true
}

func (r *BaseRepo) bumpCacheVersion(ctx context.Context, key string) bool {
	if r == nil || r.Data == nil || r.Data.rdb == nil {
		return false
	}
	var err error
	for attempt := 1; attempt <= 3; attempt++ {
		if err = r.Data.rdb.Incr(ctx, key).Err(); err == nil {
			r.Data.permissionCacheBypass.Delete(key)
			return true
		}
		if attempt < 3 {
			select {
			case <-ctx.Done():
				err = ctx.Err()
				attempt = 3
			case <-time.After(time.Duration(attempt*20) * time.Millisecond):
			}
		}
	}
	r.Data.permissionCacheBypass.Store(key, struct{}{})
	r.Log.WithContext(ctx).Errorf("刷新缓存版本失败，已旁路相关权限缓存 key=%s err=%v", key, err)
	return false
}

func (r *BaseRepo) bumpMenuVersion(ctx context.Context) {
	r.enqueuePermissionCacheInvalidation(ctx, "menu", 0)
	r.bumpCacheVersion(ctx, menuVersionKey())
}

func (r *BaseRepo) bumpTenantPackageVersion(ctx context.Context, tenantID uint32) {
	if tenantID == 0 {
		return
	}
	r.enqueuePermissionCacheInvalidation(ctx, "tenant_package", tenantID)
	r.bumpCacheVersion(ctx, tenantPackageVersionKey(tenantID))
}

func (r *BaseRepo) enqueuePermissionCacheInvalidation(ctx context.Context, scope string, tenantID uint32) {
	if r == nil || r.Data == nil || r.Data.db == nil {
		return
	}
	payload, err := json.Marshal(map[string]any{"scope": scope, "tenantId": tenantID})
	if err != nil {
		r.Log.WithContext(ctx).Errorf("序列化权限缓存失效任务失败 scope=%s tenant_id=%d err=%v", scope, tenantID, err)
		return
	}
	summary := "刷新全局菜单权限缓存版本"
	var taskTenantID *uint32
	if tenantID > 0 {
		summary = fmt.Sprintf("刷新租户 %d 套餐权限缓存版本", tenantID)
		taskTenantID = &tenantID
	}
	now := time.Now()
	idempotencyKey := permissionCacheInvalidationIdempotencyKey(scope, tenantID, now)
	systemCtx := entviewer.NewSystemContext(ctx)
	_, err = r.Data.DB(systemCtx).AsyncTask.Create().
		SetNillableTenantID(taskTenantID).
		SetTaskType(biz.AsyncTaskTypePermissionCacheInvalidate).
		SetQueue("maintenance").
		SetStatus(int32(pb.AsyncTaskStatus_ASYNC_TASK_STATUS_PENDING)).
		SetPriority(100).
		SetPayload(string(payload)).
		SetPayloadSummary(summary).
		SetIdempotencyKey(idempotencyKey).
		SetAttempts(0).
		SetMaxAttempts(10).
		SetScheduledAt(now).
		Save(systemCtx)
	if gen.IsConstraintError(err) {
		exists, lookupErr := r.Data.DB(systemCtx).AsyncTask.Query().
			Where(asynctask.IdempotencyKeyEQ(idempotencyKey)).
			Exist(systemCtx)
		if lookupErr == nil && exists {
			return
		}
	}
	if err != nil {
		r.Log.WithContext(ctx).Errorf("持久化权限缓存失效任务失败 scope=%s tenant_id=%d err=%v", scope, tenantID, err)
	}
}

func permissionCacheInvalidationIdempotencyKey(scope string, tenantID uint32, now time.Time) string {
	if tenantID > 0 {
		return fmt.Sprintf("permission-cache:%s:%d:%d", scope, tenantID, now.UTC().UnixNano())
	}
	return fmt.Sprintf("permission-cache:%s:%d", scope, now.UTC().UnixNano())
}

func (r *BaseRepo) getTenantEffectiveMenuIDsCache(ctx context.Context, tenantID uint32) ([]uint32, bool) {
	if r == nil || r.Data == nil || r.Data.rdb == nil {
		return nil, false
	}
	packageKey := tenantPackageVersionKey(tenantID)
	if _, bypass := r.Data.permissionCacheBypass.Load(menuVersionKey()); bypass {
		return nil, false
	}
	if _, bypass := r.Data.permissionCacheBypass.Load(packageKey); bypass {
		return nil, false
	}
	menuVersion, menuVersionOK := r.cacheUint64(ctx, menuVersionKey())
	packageVersion, packageVersionOK := r.cacheUint64(ctx, packageKey)
	if !menuVersionOK || !packageVersionOK {
		return nil, false
	}
	key := tenantEffectiveMenuIDsCacheKey(tenantID, menuVersion, packageVersion)
	payload, err := r.Data.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if err != redis.Nil {
			r.Log.WithContext(ctx).Warnf("读取租户有效菜单缓存失败 key=%s err=%v", key, err)
		}
		return nil, false
	}
	var ids []uint32
	if err := json.Unmarshal(payload, &ids); err != nil {
		r.Log.WithContext(ctx).Warnf("解析租户有效菜单缓存失败 key=%s err=%v", key, err)
		return nil, false
	}
	return ids, true
}

func (r *BaseRepo) setTenantEffectiveMenuIDsCache(ctx context.Context, tenantID uint32, ids []uint32) {
	if r == nil || r.Data == nil || r.Data.rdb == nil {
		return
	}
	packageKey := tenantPackageVersionKey(tenantID)
	if _, bypass := r.Data.permissionCacheBypass.Load(menuVersionKey()); bypass {
		return
	}
	if _, bypass := r.Data.permissionCacheBypass.Load(packageKey); bypass {
		return
	}
	menuVersion, menuVersionOK := r.cacheUint64(ctx, menuVersionKey())
	packageVersion, packageVersionOK := r.cacheUint64(ctx, packageKey)
	if !menuVersionOK || !packageVersionOK {
		return
	}
	key := tenantEffectiveMenuIDsCacheKey(tenantID, menuVersion, packageVersion)
	payload, err := json.Marshal(ids)
	if err != nil {
		r.Log.WithContext(ctx).Warnf("序列化租户有效菜单缓存失败 key=%s err=%v", key, err)
		return
	}
	if err := r.Data.rdb.Set(ctx, key, payload, tenantEffectiveMenuCacheTTL).Err(); err != nil {
		r.Log.WithContext(ctx).Warnf("写入租户有效菜单缓存失败 key=%s err=%v", key, err)
	}
}

type permissionCacheInvalidator struct {
	data *Data
}

func NewPermissionCacheInvalidator(data *Data) biz.PermissionCacheInvalidator {
	return &permissionCacheInvalidator{data: data}
}

func (i *permissionCacheInvalidator) InvalidateMenuPermissionCache(ctx context.Context) error {
	return i.invalidate(ctx, menuVersionKey())
}

func (i *permissionCacheInvalidator) InvalidateTenantPackagePermissionCache(ctx context.Context, tenantID uint32) error {
	if tenantID == 0 {
		return fmt.Errorf("tenant id is required")
	}
	return i.invalidate(ctx, tenantPackageVersionKey(tenantID))
}

func (i *permissionCacheInvalidator) invalidate(ctx context.Context, key string) error {
	if i == nil || i.data == nil || i.data.rdb == nil {
		return fmt.Errorf("redis client is unavailable")
	}
	if err := i.data.rdb.Incr(ctx, key).Err(); err != nil {
		return fmt.Errorf("increment permission cache version %s: %w", key, err)
	}
	i.data.permissionCacheBypass.Delete(key)
	return nil
}
