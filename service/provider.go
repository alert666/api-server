package service

import (
	"github.com/google/wire"
	v1 "github.com/qinquanliuxiang666/alertmanager/service/v1"
)

var ServiceProviderSet = wire.NewSet(
	v1.NewUserService,
	v1.NewRoleService,
	v1.NewApiServicer,
	v1.NewAlertsServicer,
	v1.NewAlertTemplateServicer,
	v1.NewChannelServicer,
	v1.NewHistoryServicer,
	v1.NewCleanDuplicateFiringer,
)
