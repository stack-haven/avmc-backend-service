//go:build wireinject
// +build wireinject

// Package main · wire.go
// Wire DI 声明（v1.4 精简）。
//
// 简化记录（参照 evie/service 风格）：
//   - 移除 provideCanQuaFetch：原 canQuaFetch 字段为死代码（set 但从未读取）。
//   - 移除 provideHealthNotifier：用 wire.Bind 直接声明 *HealthChecker 满足 biz.HealthNotifier。
//   - 保留 provideHealthCheckerWithTokenReporter：这是 wire 的反向注入惯用法
//     （TenantRegistry 在 NewTenantRegistry 阶段构造，但需要被 HealthChecker
//      在 Details() 中调用，因此 wire 必须在 HealthChecker 构造后再调
//     SetTokenReporter 把 registry 注入到 checker）。
//
// 依赖链：conf → data.ProviderSet → biz.ProviderSet → service.ProviderSet → server.ProviderSet → newApp
package main

import (
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"

	pkgHealth "backend-service/pkg/health"

	"backend-service/app/evie/tool/internal/biz"
	"backend-service/app/evie/tool/internal/conf"
	"backend-service/app/evie/tool/internal/data"
	"backend-service/app/evie/tool/internal/server"
	"backend-service/app/evie/tool/internal/service"
)

// initHealthChecker 把 TenantRegistry 注入 HealthChecker（反向注入）。
//
// 为什么需要 wire.Bind(new(biz.HealthNotifier), new(*HealthChecker))：
// *HealthChecker 同时实现 pkgHealth.Checker 和 biz.HealthNotifier，wire
// 默认不知道这层 interface 关系，需要显式声明。
func initHealthChecker(
	checker *data.HealthChecker,
	registry *biz.TenantRegistry,
) pkgHealth.Checker {
	if checker != nil && registry != nil {
		checker.SetTokenReporter(registry)
	}
	return checker
}

// wireApp 装配 evie/tool Kratos App + 后台 worker。
//
// 依赖方向严格遵 service → biz → data：
//   - biz 定义接口（VocabularySource / AuthContext / HealthNotifier）
//   - data 实现接口（NewQuaVocabularySource / *AuthInfo implements AuthContext / *HealthChecker implements HealthNotifier）
//   - wire 把各层 ProviderSet 绑在一起，interface binding 在本文件显式声明
func wireApp(
	*conf.Server, *conf.Data, *conf.Asr, *conf.Qua,
	*conf.Enhancement, *conf.TenantVocab, *conf.SystemDict, *conf.TenantRegistry,
	*conf.VocabRules,
	log.Logger,
) (*kratos.App, func(), error) {
	panic(wire.Build(
		data.ProviderSet,
		biz.ProviderSet,
		service.ProviderSet,
		server.ProviderSet,
		// *HealthChecker 同时实现 pkgHealth.Checker 和 biz.HealthNotifier，
		// 此绑定让 wire 知道用同一实例满足两个接口。
		wire.Bind(new(biz.HealthNotifier), new(*data.HealthChecker)),
		initHealthChecker,            // 反向注入：TenantRegistry → HealthChecker
		biz.NewVocabSyncerWithAuth,   // 内部已调 AttachLazySync(builder)
		newApp,
	))
}
