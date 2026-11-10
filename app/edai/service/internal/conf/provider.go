package conf

import "github.com/google/wire"

// ProviderSet 从 Bootstrap 中拆出各子配置，供 wire 注入。
//
// conf.Bootstrap 是 backend-service/api/common/conf 的别名，wire 无法自动拆分
// 嵌套消息，因此显式提供各子配置。
var ProviderSet = wire.NewSet(ProvideServer, ProvideData, ProvideMiddleware)

func ProvideServer(bc *Bootstrap) *Server {
	if bc == nil {
		return nil
	}
	return bc.Server
}

func ProvideData(bc *Bootstrap) *Data {
	if bc == nil {
		return nil
	}
	return bc.Data
}

// ProvideMiddleware 取 HTTP 中间件配置（含 JWT 签名密钥）。
func ProvideMiddleware(bc *Bootstrap) *Middleware {
	if bc == nil || bc.Server == nil || bc.Server.Http == nil {
		return nil
	}
	return bc.Server.Http.Middleware
}
