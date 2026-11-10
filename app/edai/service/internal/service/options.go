package service

import (
	"backend-service/app/edai/service/internal/conf"
)

// NewSystemOptions 从配置构造系统信息参数。
func NewSystemOptions(bc *conf.Bootstrap) SystemOptions {
	opts := SystemOptions{}
	if bc != nil && bc.Data != nil && bc.Data.Database != nil {
		opts.DBDriver = bc.Data.Database.Driver
	}
	if bc != nil && bc.Server != nil && bc.Server.Http != nil {
		opts.HTTPAddr = bc.Server.Http.Addr
	}
	return opts
}
