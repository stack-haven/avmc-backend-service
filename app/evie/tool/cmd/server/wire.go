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

// wireApp 装配 evie/tool Kratos App + 后台 worker。
//
// M5/M6 依赖链：
//   conf.SystemDict → VocabularyBuilder（加载 system.json）
//   conf.VocabRules → Normalizer
//   conf.Enhancement → PolicyFromConf + EnhancementPipeline（registry + observers）
//   conf.Qua → QuaClient + QuaVocabularySource（adapter）
//   conf.TenantRegistry → TenantRegistry
//   TenantRegistry + QuaVocabularySource + Normalizer + VocabularyBuilder → VocabSyncer
//   VocabularyBuilder + Pipeline + Policy → EnhancementUsecase
//   EnhancementUsecase → EnhancementService
//   EnhancementService → server (HTTP + gRPC)
//   VocabSyncer → 通过 newApp 的 BeforeStart 启动后台 worker
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
		provideCanQuaFetch,           // 注入给 VocabSyncer
		biz.NewVocabSyncerWithAuth,   // 用带 auth 检查的 syncer 构造器，内部调 AttachLazySync
		newApp,
	))
}
