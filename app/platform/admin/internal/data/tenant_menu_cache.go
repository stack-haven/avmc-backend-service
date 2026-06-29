package data

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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

func (r *BaseRepo) cacheUint64(ctx context.Context, key string) uint64 {
	if r == nil || r.Data == nil || r.Data.rdb == nil {
		return 0
	}
	value, err := r.Data.rdb.Get(ctx, key).Uint64()
	if err != nil && err != redis.Nil {
		r.Log.WithContext(ctx).Warnf("读取缓存版本失败 key=%s err=%v", key, err)
	}
	return value
}

func (r *BaseRepo) bumpCacheVersion(ctx context.Context, key string) {
	if r == nil || r.Data == nil || r.Data.rdb == nil {
		return
	}
	if err := r.Data.rdb.Incr(ctx, key).Err(); err != nil {
		r.Log.WithContext(ctx).Warnf("刷新缓存版本失败 key=%s err=%v", key, err)
	}
}

func (r *BaseRepo) bumpMenuVersion(ctx context.Context) {
	r.bumpCacheVersion(ctx, menuVersionKey())
}

func (r *BaseRepo) bumpTenantPackageVersion(ctx context.Context, tenantID uint32) {
	if tenantID == 0 {
		return
	}
	r.bumpCacheVersion(ctx, tenantPackageVersionKey(tenantID))
}

func (r *BaseRepo) getTenantEffectiveMenuIDsCache(ctx context.Context, tenantID uint32) ([]uint32, bool) {
	if r == nil || r.Data == nil || r.Data.rdb == nil {
		return nil, false
	}
	menuVersion := r.cacheUint64(ctx, menuVersionKey())
	packageVersion := r.cacheUint64(ctx, tenantPackageVersionKey(tenantID))
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
	menuVersion := r.cacheUint64(ctx, menuVersionKey())
	packageVersion := r.cacheUint64(ctx, tenantPackageVersionKey(tenantID))
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
