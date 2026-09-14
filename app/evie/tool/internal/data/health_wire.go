// Package data · health_wire.go
// HealthChecker 的 wire 集成（反向注入 provider + interface bindings）。
//
// 业务背景：
//   HealthChecker.Details() 报告 expiring_tenants / expired_tenants，
//   需要查询 TenantRegistry。但 TenantRegistry 由 biz.NewTenantRegistry 构造。
//   wire 构造顺序：NewHealthChecker → NewTenantRegistry → NewVocabSyncerWithAuth。
//   本文件提供：
//     1. NewHealthCheckerWithTokenReporter — 反向注入 TenantRegistry → HealthChecker
//     2. wire.Bind 声明 *HealthChecker 满足 pkgHealth.Checker 和 biz.HealthNotifier
//
// 这些声明放在 data 包（实现方）符合 Kratos 架构惯例：
// "实现方声明自己满足哪些接口"（与 Java implements / TypeScript implements 类似）。
package data

import (

	"backend-service/app/evie/tool/internal/biz"
	pkgHealth "backend-service/pkg/health"
)

// NewHealthCheckerWithReporter 把 TenantRegistry 注入 HealthChecker（反向注入）。
//
// 为什么需要反向注入：
//   HealthChecker.Details() 要查 token 过期状态（expiring_tenants /
//   expired_tenants），数据来自 biz.TenantRegistry。但 wire 的依赖图
//   是单向的：NewHealthChecker 不感知 TenantRegistry。
//
//   本 provider 让 wire 在 TenantRegistry 构造完后调 SetTokenReporter，
//   完成"延迟反向注入"，避免在 NewHealthChecker 阶段就需要 *TenantRegistry。
//
// 返回 pkgHealth.Checker 接口（而非 *HealthChecker concrete）：
//   - 避免与 NewHealthChecker 的返回值冲突（wire 不允许同一 concrete 类型有多个 provider）
//   - server.NewHTTPServer 需要的正是 pkgHealth.Checker 接口
//
// 调用顺序（wire 自动）：
//   1. data.NewHealthChecker(...) → *HealthChecker (无 token reporter)
//   2. biz.NewTenantRegistry(...) → *TenantRegistry
//   3. NewHealthCheckerWithReporter(checker, registry) → pkgHealth.Checker (含 reporter)
//   4. biz.NewVocabSyncerWithAuth(..., healthChecker) 收到已被 SetTokenReporter 的 *HealthChecker
func NewHealthCheckerWithReporter(
	checker *HealthChecker,
	registry *biz.TenantRegistry,
) pkgHealth.Checker {
	if checker != nil && registry != nil {
		checker.SetTokenReporter(registry)
	}
	return checker
}
