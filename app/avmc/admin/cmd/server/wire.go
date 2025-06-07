//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package main

import (
	"backend-service/app/avmc/admin/internal/biz"
	"backend-service/app/avmc/admin/internal/conf"
	"backend-service/app/avmc/admin/internal/data"
	"backend-service/app/avmc/admin/internal/server"
	"backend-service/app/avmc/admin/internal/service"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

// wireApp init kratos application.
func wireApp(*conf.Server, *conf.Data, log.Logger) (*kratos.App, func(), error) {
	panic(wire.Build(server.ProviderSet, data.ProviderSet, biz.ProviderSet, service.ProviderSet, newApp))
}
