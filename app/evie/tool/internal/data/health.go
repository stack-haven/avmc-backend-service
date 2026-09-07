// Package data · health.go
// evie/tool 健康检查器（M9 收口）。
//
// 检查范围：
//   - Redis：Ping
//   - Qua HTTP：HEAD baseURL
//   - ASR providers：遍历 enabled providers
//
// 不做：磁盘 / GPU / LLM provider（evie/tool 不依赖）。
// VocabSyncer 失败不阻断 ready（warn 级别）。
package data

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	asrPkg "backend-service/pkg/asr"
	pkgHealth "backend-service/pkg/health"
)

// HealthChecker 聚合多个 dependency 检查。
type HealthChecker struct {
	rdb           *redis.Client
	qua           *quaFetcher
	asrReg        *asrPkg.ProviderRegistry
	tokenReporter TokenExpiryReporter
	mu            sync.RWMutex
	lastSync      time.Time
	lastError     string
	syncMode      string
}

// 编译期断言 HealthChecker 实现 pkgHealth.Checker。
var _ pkgHealth.Checker = (*HealthChecker)(nil)

// NewHealthChecker 创建 evie/tool 健康检查器。
//
// 返回 pkgHealth.Checker 接口以简化 Wire 装配（避免 wire.Bind）
func NewHealthChecker(rdb *redis.Client, qua QuaFetcher, reg *asrPkg.ProviderRegistry) pkgHealth.Checker {
	var q *quaFetcher
	if qua != nil {
		if qc, ok := qua.(*quaFetcher); ok {
			q = qc
		}
	}
	return &HealthChecker{rdb: rdb, qua: q, asrReg: reg}
}

// SetSyncState 由 VocabSyncer 在每次同步完成时回调，更新内部状态。
func (c *HealthChecker) SetSyncState(last time.Time, errMsg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastSync = last
	c.lastError = errMsg
}

// SetSyncMode 上报后台词库同步模式（lazy_only / background）。
//
// 编译期保证：*HealthChecker 满足 biz.HealthNotifier。
func (c *HealthChecker) SetSyncMode(mode string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.syncMode = mode
}

// TokenExpiryReporter 可选注入：由 biz 提供租户 token 过期状态。
//
// biz.TenantRegistry 本身已实现此接口；不设置则 health 输出中不包含
// expiring_tenants / expired_tenants / expired_token_tenants 字段。
type TokenExpiryReporter interface {
	ExpiringTenants(threshold time.Duration) []string
	ExpiredTenants() []string
}

// SetTokenReporter 注入 token 过期报告器。
func (c *HealthChecker) SetTokenReporter(r TokenExpiryReporter) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tokenReporter = r
}

// Ready 检查所有依赖（带 2s 总超时）。
func (c *HealthChecker) Ready(ctx context.Context) error {
	if c == nil {
		return errors.New("health: nil checker")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	var errs []error
	if c.rdb == nil {
		errs = append(errs, errors.New("redis: not configured"))
	} else if err := c.rdb.Ping(ctx).Err(); err != nil {
		errs = append(errs, fmt.Errorf("redis: %w", err))
	}

	if c.qua != nil && c.qua.BaseURL() != "" {
		pingCtx, cancelPing := context.WithTimeout(ctx, 1*time.Second)
		pingErr := c.qua.Ping(pingCtx)
		cancelPing()
		if pingErr != nil {
			errs = append(errs, fmt.Errorf("qua: %w", pingErr))
		}
	}

	if c.asrReg != nil {
		for _, name := range c.asrReg.Names() {
			p, err := c.asrReg.Get(name)
			if err != nil || p == nil {
				errs = append(errs, fmt.Errorf("asr[%s]: %v", name, err))
				continue
			}
			_ = p.Capabilities()
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// Details 返回诊断数据（用于 ready 详情输出）。
func (c *HealthChecker) Details(_ context.Context) map[string]any {
	c.mu.RLock()
	reporter := c.tokenReporter
	c.mu.RUnlock()

	details := map[string]any{
		"redis":           c.rdb != nil,
		"qua":             c.qua != nil && c.qua.BaseURL() != "",
		"asr":             c.asrReg != nil,
		"vocab_sync_mode": c.syncMode,
	}
	if c.asrReg != nil {
		details["asr_providers"] = c.asrReg.Names()
	}
	if !c.lastSync.IsZero() {
		details["vocab_last_sync"] = c.lastSync.Format(time.RFC3339)
	}
	if c.lastError != "" {
		details["vocab_last_error"] = c.lastError
	}
	if reporter != nil {
		// 过期 token 状态（供运维监控）
		if exp := reporter.ExpiringTenants(time.Hour); len(exp) > 0 {
			details["expiring_tenants"] = exp
		}
		if expired := reporter.ExpiredTenants(); len(expired) > 0 {
			details["expired_tenants"] = expired
		}
	}
	return details
}
