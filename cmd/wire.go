// go:build wireinject
//go:build wireinject
// +build wireinject

package cmd

import (
	"github.com/google/wire"
	"github.com/alert666/api-server/base/app"
	"github.com/alert666/api-server/base/data"
	"github.com/alert666/api-server/base/middleware"
	"github.com/alert666/api-server/base/router"
	"github.com/alert666/api-server/base/server"
	"github.com/alert666/api-server/controller"
	"github.com/alert666/api-server/pkg"
	"github.com/alert666/api-server/service"
	"github.com/alert666/api-server/store"
)

func InitApplication() (*app.Application, func(), error) {
	panic(wire.Build(
		data.DataProviderSet,
		pkg.PkgProviderSet,
		store.StoreProviderSet,
		service.ServiceProviderSet,
		controller.ControllerProviderSet,
		middleware.MiddlewareProviderSet,
		router.RouterProviderSet,
		server.ServerProviderSet,
		app.AppProviderSet,
	))
}
