// Package server 聚合 evie/tool 的 HTTP/gRPC transport Provider。
package server

import "github.com/google/wire"

// ProviderSet server providers（M0 阶段仅占位，M7/M9 阶段按需填充 service 注册）。
var ProviderSet = wire.NewSet(
	NewGRPCServer,
	NewHTTPServer,
)