package controller

import "github.com/google/wire"

var ControllerProviderSet = wire.NewSet(
	NewUserController,
	NewRoleController,
	NewApiController,
	NewAlertManagerController,
	NewAlertTemplateController,
	NewAlertChannelController,
	NewAlertHistoryController,
	NewAlertSilenceController,
)
