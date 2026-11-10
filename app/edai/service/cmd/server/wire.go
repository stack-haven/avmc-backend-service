//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package main

import (
	"backend-service/app/edai/service/internal/biz"
	"backend-service/app/edai/service/internal/conf"
	"backend-service/app/edai/service/internal/data"
	"backend-service/app/edai/service/internal/server"
	"backend-service/app/edai/service/internal/service"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

// wireApp init kratos application.
func wireApp(*conf.Bootstrap, log.Logger) (*kratos.App, func(), error) {
	panic(wire.Build(
		conf.ProviderSet,
		server.ProviderSet,
		data.ProviderSet,
		data.AuthProviderSet,
		biz.ProviderSet,
		service.ProviderSet,
		newApp,
	))
}
