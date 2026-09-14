//go:build wireinject
// +build wireinject

// Package main · wire.go
// Wire DI 声明（v1.5 — 与 evie/service 风格对齐）。
//
// 所有 wire 表达式（provider / wire.Bind / reverse injection）已下沉到各层
// ProviderSet，本文件仅做组合：
//   - data.ProviderSet：基础设施工厂（含 HealthChecker interface binding + reverse injection）
//   - biz.ProviderSet：业务用例工厂（含 NewVocabSyncerWithAuth）
//   - service.ProviderSet：Kratos Service 适配层
//   - server.ProviderSet：HTTP/gRPC server 工厂
//
// 依赖方向严格遵 service → biz → data（单向依赖）。
package main

import (
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"

	"backend-service/app/evie/tool/internal/biz"
	"backend-service/app/evie/tool/internal/conf"
	"backend-service/app/evie/tool/internal/data"
	"backend-service/app/evie/tool/internal/server"
	"backend-service/app/evie/tool/internal/service"
)

// wireApp 装配 evie/tool Kratos App + 后台 worker。
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
		newApp,
	))
}
