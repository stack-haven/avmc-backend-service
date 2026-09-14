// Package data · health.go
// evie/tool 健康检查器（v1.7 减法后）。
//
// 检查范围（Ready）：
//   - Redis：Ping
//   - Qua HTTP：HEAD baseURL
//   - ASR providers：遍历 enabled providers
//
// 检查范围（Details）：
//   - 上述依赖是否配置 + asr_providers 列表
//
// v1.7 减法：删除与后台 sync 相关的方法/字段（SetSyncState / SetSyncMode /
// lastSync / lastError / syncMode / mu），全部由 pkg/health 自动探测 DetailsProvider。
package data

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	asrPkg "backend-service/pkg/asr"
	pkgHealth "backend-service/pkg/health"
)

// HealthChecker 聚合多个 dependency 检查（Ready + Details）。
type HealthChecker struct {
	rdb    *redis.Client
	qua    *quaFetcher
	asrReg *asrPkg.ProviderRegistry
}

// 编译期断言 HealthChecker 实现 pkgHealth.Checker 和 DetailsProvider。
var (
	_ pkgHealth.Checker        = (*HealthChecker)(nil)
	_ pkgHealth.DetailsProvider = (*HealthChecker)(nil)
)

// NewHealthChecker 创建 evie/tool 健康检查器。
//
// 返回 concrete *HealthChecker，由 data.ProviderSet 的 wire.Bind 让其同时满足
// pkgHealth.Checker（server.NewHTTPServer 需要）。
func NewHealthChecker(rdb *redis.Client, qua QuaFetcher, reg *asrPkg.ProviderRegistry) *HealthChecker {
	var q *quaFetcher
	if qua != nil {
		if qc, ok := qua.(*quaFetcher); ok {
			q = qc
		}
	}
	return &HealthChecker{rdb: rdb, qua: q, asrReg: reg}
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

// Details 返回诊断数据（pkg/health 自动探测 DetailsProvider 并输出）。
func (c *HealthChecker) Details(_ context.Context) map[string]any {
	details := map[string]any{
		"redis": c.rdb != nil,
		"qua":   c.qua != nil && c.qua.BaseURL() != "",
		"asr":   c.asrReg != nil,
	}
	if c.asrReg != nil {
		details["asr_providers"] = c.asrReg.Names()
	}
	return details
}
