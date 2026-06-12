// go:build wireinject
//go:build wireinject
// +build wireinject

package cmd

import (
	"github.com/alert666/api-server/base/conf"
	"github.com/google/wire"
	"github.com/alert666/api-server/base/app"
	"github.com/alert666/api-server/base/data"
	"github.com/alert666/api-server/base/middleware"
	"github.com/alert666/api-server/base/router"
	"github.com/alert666/api-server/base/server"
	"github.com/alert666/api-server/controller"
	"github.com/alert666/api-server/pkg"
	svc "github.com/alert666/api-server/service"
	"github.com/alert666/api-server/store"
	grpchandler "github.com/alert666/api-server/grpc/handler"
	grpcserver "github.com/alert666/api-server/grpc/server"
)

// NewGRPCBindAddress 提供 gRPC 监听地址
func NewGRPCBindAddress() string {
	return conf.GetGRPCBind()
}

func InitApplication() (*app.Application, func(), error) {
	panic(wire.Build(
		data.DataProviderSet,
		pkg.PkgProviderSet,
		store.StoreProviderSet,
		NewGRPCBindAddress,
		svc.ServiceProviderSet,
		controller.ControllerProviderSet,
		middleware.MiddlewareProviderSet,
		router.RouterProviderSet,
		grpchandler.HandlerProviderSet,
		grpcserver.GRPCServerProviderSet,
		server.ServerProviderSet,
		app.AppProviderSet,
	))
}
