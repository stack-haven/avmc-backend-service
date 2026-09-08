//go:build wireinject
// +build wireinject

// Package main · wire.go
// Wire DI 声明。Bootstrap 各 section 分别注入。
package main

import (
	"context"

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

// provideCanQuaFetch 注入 VocabSyncer 的 AuthInfo 检测函数。
//
// 为什么需要：vocab warmup 在启动期执行，此时 ctx 是 background，
// 调 qua 会 401（缺 AuthInfo）。这里通过 biz.AuthFrom 检测 ctx 中是否有
// AuthContext，有才允许 qua 调用。
//
// 为什么用 biz.AuthFrom 而不是 data.AuthInfoFromContext：data 不能 import biz，
// 但 biz.AuthFrom 是抽象接口，与 data 层实现解耦。
func provideCanQuaFetch() func(ctx context.Context) bool {
	return func(ctx context.Context) bool {
		_, ok := biz.AuthFrom(ctx)
		return ok
	}
}

// provideHealthNotifier 将 pkgHealth.Checker 适配为 biz.HealthNotifier。
func provideHealthNotifier(checker pkgHealth.Checker) biz.HealthNotifier {
	if checker == nil {
		return nil
	}
	if n, ok := checker.(biz.HealthNotifier); ok {
		return n
	}
	return nil
}

// provideHealthCheckerWithTokenReporter 把 *biz.TenantRegistry 注入 HealthChecker。
//
// 设计：HealthChecker 的 Details() 需要暴露 token 过期状态（expiring_tenants /
// expired_tenants），但 TenantRegistry 由 biz 包构造，wire 注入在 HealthChecker
// 之后。这个 provider 把"反向注入"声明为依赖关系，wire 会自动按依赖顺序调用：
//  1. NewHealthChecker 构造 HealthChecker
//  2. NewTenantRegistry 构造 TenantRegistry
//  3. 本 provider 触发 SetTokenReporter（如果 checker 是 *data.HealthChecker）
//  4. server.NewHTTPServer 接收的 checker 已被注入 reporter
//
// 反向注入是 wire 友好的写法：避免手动修改 wire_gen.go。
func provideHealthCheckerWithTokenReporter(
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
// M5/M6 依赖链：
//
//	conf.SystemDict → VocabularyBuilder（加载 system.json）
//	conf.VocabRules → Normalizer
//	conf.Enhancement → PolicyFromConf + EnhancementPipeline（registry + observers）
//	conf.Qua → QuaClient + QuaVocabularySource（adapter）
//	conf.TenantRegistry → TenantRegistry
//	TenantRegistry + QuaVocabularySource + Normalizer + VocabularyBuilder → VocabSyncer
//	VocabularyBuilder + Pipeline + Policy → EnhancementUsecase
//	EnhancementUsecase → EnhancementService
//	EnhancementService → server (HTTP + gRPC)
//	VocabSyncer → 通过 newApp 的 BeforeStart 启动后台 worker
//
// 依赖方向严格遵 service → biz → data：
//   - biz 定义接口（VocabularySource / AuthContext）
//   - data 实现接口（NewQuaVocabularySource / *AuthInfo implements AuthContext）
//   - wire 把 data.ProviderSet、biz.ProviderSet、service.ProviderSet、server.ProviderSet 绑在一起
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
		provideCanQuaFetch,                       // 注入给 VocabSyncer
		provideHealthNotifier,                    // 将 HealthChecker 适配为 biz.HealthNotifier
		provideHealthCheckerWithTokenReporter,    // 反向注入 TenantRegistry → HealthChecker
		biz.NewVocabSyncerWithAuth,               // 内部已调 AttachLazySync(builder)
		newApp,
	))
}
