//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package main

import (
	"backend-service/app/evie/service/internal/biz"
	"backend-service/app/evie/service/internal/conf"
	"backend-service/app/evie/service/internal/data"
	"backend-service/app/evie/service/internal/server"
	"backend-service/app/evie/service/internal/service"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

// wireApp init kratos application.
func wireApp(*conf.Server, *conf.Client, *conf.Data, *conf.OSS, log.Logger) (*kratos.App, func(), error) {
	panic(wire.Build(server.ProviderSet, data.ProviderSet, biz.ProviderSet, service.ProviderSet, newApp))
}
