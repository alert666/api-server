package controller

import (
	"github.com/alert666/api-server/base/bind"
	v1 "github.com/alert666/api-server/service/v1"
	"github.com/gin-gonic/gin"
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
	bind.ResponseOnlySuccess(c, receiver.alertService.SendAlert, bind.BindTypeQuery, bind.BindTypeJson)
}
