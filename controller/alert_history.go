package controller

import (
	"github.com/alert666/api-server/base/bind"
	v1 "github.com/alert666/api-server/service/v1"
	"github.com/gin-gonic/gin"
)

type AlertHistoryController interface {
	QueryAlertHistory(c *gin.Context)
	ListAlertHistory(c *gin.Context)
	UpdateAlertHistory(c *gin.Context)
	GetTenantFiringCounts(c *gin.Context)
	GetAlertNameOptions(c *gin.Context)
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
// @Summary 查询 AlertHistory
// @Description 使用 ID 查询告警历史详情
// @Tags AlertHistory 管理
// @Accept json
// @Produce json
// @Param id path int true "告警历史ID"
// @Success 200 {object} types.Response "创建成功"
// @Router /api/v1/alertHistory/:id [get]
func (recevicer *alertHistoryController) QueryAlertHistory(c *gin.Context) {
	bind.ResponseWithData(c, recevicer.alertHistoryService.QueryHistory, bind.BindTypeUri)
}

// @Summary 获取所有 AlertHistory
// @Description 获取所有 AlertHistory
// @Tags AlertHistory 管理
// @Accept json
// @Produce json
// @Param data query types.AlertHistoryListRequest true "查询请求参数"
// @Success 200 {object} types.Response{} "查询成功"
// @Router /api/v1/alertHistory [get]
func (receiver *alertHistoryController) ListAlertHistory(c *gin.Context) {
	bind.ResponseWithData(c, receiver.alertHistoryService.ListHistory, bind.BindTypeQuery)
}

// @Summary 更新 AlertHistory 状态
// @Description 更新 AlertHistory 状态
// @Tags AlertHistory 管理
// @Accept json
// @Produce json
// @Param id path int true "告警历史ID"
// @Param data body types.AlertHistoryUpdateRequest true "更新请求参数"
// @Success 200 {object} types.Response{} "更新成功"
// @Router /api/v1/alertHistory/:id [put]
func (receiver *alertHistoryController) UpdateAlertHistory(c *gin.Context) {
	bind.ResponseOnlySuccess(c, receiver.alertHistoryService.UpdateHistory, bind.BindTypeJson, bind.BindTypeUri)
}

// @Summary 获取 AlertHistory 告警状态数量
// @Description 更新 获取 AlertHistory 告警状态数量
// @Tags AlertHistory 管理
// @Accept json
// @Produce json
// @Success 200 {object} types.Response{} "询成功"
// @Router /api/v1/alertHistory/firingCount [get]
func (receiver *alertHistoryController) GetTenantFiringCounts(c *gin.Context) {
	bind.ResponseWithDataNoBind(c, receiver.alertHistoryService.GetTenantFiringCounts)
}

// @Summary 获取 AlertHistory aletName options
// @Description 获取 AlertHistory aletName options
// @Tags AlertHistory 管理
// @Accept json
// @Produce json
// @Success 200 {object} types.Response{} "查询成功"
// @Router /api/v1/alertHistory/alertNameOptions [get]
func (receiver *alertHistoryController) GetAlertNameOptions(c *gin.Context) {
	bind.ResponseWithDataNoBind(c, receiver.alertHistoryService.GetAlertNameOptions)
}
