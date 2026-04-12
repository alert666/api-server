package service

import (
	v1 "github.com/alert666/api-server/service/v1"
	"github.com/google/wire"
)

var ServiceProviderSet = wire.NewSet(
	v1.NewUserService,
	v1.NewRoleService,
	v1.NewApiServicer,
	v1.NewTenantServicer,
	v1.NewAlertsServicer,
	v1.NewAlertTemplateServicer,
	v1.NewChannelServicer,
	v1.NewHistoryServicer,
	v1.NewAlertSilenceServicer,
	v1.NewCleanDuplicateFiringer,
	v1.NewCleanExpiredSilencer,
)
