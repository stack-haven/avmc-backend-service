package main

import (
	"backend-service/pkg/conf"

	"github.com/google/wire"
)

// confProviderSet 从 Bootstrap 中拆出各子配置，供 wire 注入。
//
// 为什么放在 cmd/server 而不是 pkg/conf：
//
//	`pkg/conf` 只承载共享配置**类型**（由 proto/common/conf 生成），
//	而「如何把一个 Bootstrap 拆给各组件」是本服务的装配决策。
//	平台/evie/ai/version 选择在 main.go 里直接拆（wireApp 收子配置），
//	edai 选择用 ProviderSet 在 wire 图里拆 —— 这是服务特有编排，不应下沉到共享包。
//
// wire 无法自动拆分嵌套消息，因此显式提供各子配置。
var confProviderSet = wire.NewSet(provideConfServer, provideConfData, provideConfMiddleware)

func provideConfServer(bc *conf.Bootstrap) *conf.Server {
	if bc == nil {
		return nil
	}
	return bc.Server
}

func provideConfData(bc *conf.Bootstrap) *conf.Data {
	if bc == nil {
		return nil
	}
	return bc.Data
}

// provideConfMiddleware 取 HTTP 中间件配置（含 JWT 签名密钥）。
func provideConfMiddleware(bc *conf.Bootstrap) *conf.Middleware {
	if bc == nil || bc.Server == nil || bc.Server.Http == nil {
		return nil
	}
	return bc.Server.Http.Middleware
}
