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

// ReceiveAlerts 接收 Alertmanager 告警
// @Summary 接收 Alertmanager 告警
// @Description 作为 Alertmanager Webhook 回调接口，接收并持久化告警数据。支持同时绑定 query 参数（templateName）和 JSON body。
// @Tags 告警管理
// @Accept json
// @Produce json
// @Param templateName query string true "告警模板名称"
// @Param data body types.AlertReceiveReq true "Alertmanager Webhook JSON 数据"
// @Success 200 {object} types.Response "接收成功"
// @Router /api/v1/alerts [post]
func (receiver *alertManagerController) ReceiveAlerts(c *gin.Context) {
	bind.ResponseOnlySuccess(c, receiver.alertService.SendAlert, bind.BindTypeQuery, bind.BindTypeJson)
}
