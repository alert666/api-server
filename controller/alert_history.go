package controller

import (
	v1 "github.com/alert666/api-server/service/v1"
	"github.com/gin-gonic/gin"
)

type AlertHistoryController interface {
	QueryAlertHistory(c *gin.Context)
	ListAlertHistory(c *gin.Context)
}

type alertHistoryController struct {
	alertHistoryService v1.AlertHistoryServicer
}

func NewAlertHistoryController(alertHistoryService v1.AlertHistoryServicer) AlertHistoryController {
	return &alertHistoryController{
		alertHistoryService: alertHistoryService,
	}
}

// ListAlertHistory 查询 AlertHistory
// @Summary 创建 AlertHistory
// @Description 创建 AlertHistory
// @Tags AlertHistory 管理
// @Accept json
// @Produce json
// @Param data body types.IDRequest true "创建请求参数"
// @Success 200 {object} types.Response "创建成功"
// @Router /api/v1/alertHistory/:id [get]
func (recevicer *alertHistoryController) QueryAlertHistory(c *gin.Context) {
	ResponseWithData(c, recevicer.alertHistoryService.QueryHistory, bindTypeUri)
}

// @Summary 获取所有 AlertHistory
// @Description 获取所有 AlertHistory
// @Tags AlertHistory 管理
// @Accept json
// @Produce json
// @Success 200 {object} types.Response{data=types.AlertHistoryListResponse} "查询成功"
// @Router /api/v1/alertHistory [get]
func (receiver *alertHistoryController) ListAlertHistory(c *gin.Context) {
	ResponseWithData(c, receiver.alertHistoryService.ListHistory, bindTypeQuery)
}
