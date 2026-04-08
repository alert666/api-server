package controller

import (
	"github.com/gin-gonic/gin"
	v1 "github.com/alert666/api-server/service/v1"
)

type AlertManagerController interface {
	ReceiveAlerts(c *gin.Context)
}

type alertManagerController struct {
	alertService v1.AlertsServicer
}

func NewAlertManagerController(alertService v1.AlertsServicer) AlertManagerController {
	return &alertManagerController{
		alertService: alertService,
	}
}

func (receiver *alertManagerController) ReceiveAlerts(c *gin.Context) {
	ResponseOnlySuccess(c, receiver.alertService.SendAlert, bindTypeQuery, bindTypeJson)
}
