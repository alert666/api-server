package controller

import (
	v1 "github.com/alert666/api-server/service/v1"
	"github.com/gin-gonic/gin"
)

type AlertHistoryController interface {
	QueryAlertHistory(c *gin.Context)
	ListAlertHistory(c *gin.Context)
	UpdateAlertHistory(c *gin.Context)
	GetTenantFiringCounts(c *gin.Context)
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

// @Summary 更新 AlertHistory 状态
// @Description 更新 AlertHistory 状态
// @Tags AlertHistory 管理
// @Accept json
// @Produce json
// @Success 200 {object} types.Response{data=types.AlertHistoryUpdateRequest} "更新成功"
// @Router /api/v1/alertHistory [put]
func (receiver *alertHistoryController) UpdateAlertHistory(c *gin.Context) {
	ResponseOnlySuccess(c, receiver.alertHistoryService.UpdateHistory, bindTypeJson, bindTypeUri)
}

// @Summary 获取 AlertHistory 告警状态数量
// @Description 更新 获取 AlertHistory 告警状态数量
// @Tags AlertHistory 管理
// @Accept json
// @Produce json
// @Success 200 {object} types.Response{data=[]types.TenantFiringCount} "更新成功"
// @Router /api/v1/alertHistory/firingCount [get]
func (receiver *alertHistoryController) GetTenantFiringCounts(c *gin.Context) {
	ResponseWithDataNoBind(c, receiver.alertHistoryService.GetTenantFiringCounts)
}
