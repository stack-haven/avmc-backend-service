// Package conf 为配置类型的别名层。
//
// EDAI 直接复用 backend-service/api/common/conf（由 proto/common/conf 生成），
// 不再像 evie 那样把生成的 pb.go 复制一份到服务内，减少一次生成与同步成本。
package conf

import (
	commonconf "backend-service/api/common/conf"
)

type (
	Bootstrap  = commonconf.Bootstrap
	Server     = commonconf.Server
	Client     = commonconf.Client
	Data       = commonconf.Data
	OSS        = commonconf.OSS
	Middleware = commonconf.Middleware

	// 嵌套消息：Go 的类型别名不会带入嵌套类型，需逐个导出。
	Server_HTTP      = commonconf.Server_HTTP
	Server_GRPC      = commonconf.Server_GRPC
	Server_HTTP_CORS = commonconf.Server_HTTP_CORS
	Data_Database    = commonconf.Data_Database
	Data_Redis       = commonconf.Data_Redis
	Middleware_Auth  = commonconf.Middleware_Auth
)
