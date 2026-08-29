package service

import (
	"github.com/alert666/api-server/base/config"
	v1 "github.com/alert666/api-server/service/v1"
	"github.com/google/wire"
)

var ServiceProviderSet = wire.NewSet(
	wire.Bind(new(v1.DataTunnelServicer), new(*v1.DataTunnelService)),
	wire.Bind(new(v1.ClusterProber), new(*v1.DataTunnelService)),
	config.GetKubernetesEvents,
	v1.NewUserService,
	v1.NewRoleService,
	v1.NewApiServicer,
	v1.NewTenantServicer,
	v1.NewAlertsServicer,
	v1.NewAlertTemplateServicer,
	v1.NewChannelServicer,
	v1.NewCleanStaleCacher,
	v1.NewHistoryServicer,
	v1.NewAlertSilenceServicer,
	v1.NewCleanDuplicateFiringer,
	v1.NewCleanExpiredSilencer,
	v1.NewalertInhibit,
	v1.NewCacheAlertNameOptioner,
	v1.NewDataTunnelService,
	v1.NewKubernetesEventServicer,
	v1.NewIDCMetrics,
	v1.NewIDCHeartbeat,
)
