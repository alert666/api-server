package controller

import (
	"github.com/gin-gonic/gin"
	v1 "github.com/qinquanliuxiang666/alertmanager/service/v1"
)

type AlertSilenceController interface {
	CreateAlertSilence(c *gin.Context)
	DeleteAlertSilence(c *gin.Context)
	QueryAlertSilence(c *gin.Context)
	ListAlertSilence(c *gin.Context)
}

type alertSilenceController struct {
	alertSilenceService v1.AlertSilenceServicer
}

func NewAlertSilenceController(alertSilenceService v1.AlertSilenceServicer) AlertSilenceController {
	return &alertSilenceController{
		alertSilenceService: alertSilenceService,
	}
}

// CreateApi 创建 AlerSilence
// @Summary 创建 AlerSilence
// @Description 创建 AlerSilence
// @Tags AAlerSilence 管理
// @Accept json
// @Produce json
// @Param data body types.AlertSilenceCreateRequest true "创建请求参数"
// @Success 200 {object} types.Response "创建成功"
// @Router /api/v1/alertSilence [post]
func (receiver *alertSilenceController) CreateAlertSilence(c *gin.Context) {
	ResponseOnlySuccess(c, receiver.alertSilenceService.CreateSilence, bindTypeJson)
}

// DeleteApi 删除 AlerSilence
// @Summary 删除 AlerSilence
// @Description 删除 AlerSilence
// @Tags AlerSilence 管理
// @Accept json
// @Produce json
// @Param data body types.IDRequest true "删除请求参数"
// @Success 200 {object} types.Response "删除成功"
// @Router /api/v1/alertSilence/:id [delete]
func (receiver *alertSilenceController) DeleteAlertSilence(c *gin.Context) {
	ResponseOnlySuccess(c, receiver.alertSilenceService.DeleteSilence, bindTypeUri)
}

// QueryApi 查询 AlerSilence
// @Summary 查询 AlerSilence
// @Description 查询 AlerSilence
// @Tags AlerSilence 管理
// @Accept json
// @Produce json
// @Param data body types.IDRequest true "查询请求参数"
// @Success 200 {object} types.Response{data=model.AlertSilence} "查询成功"
// @Router /api/v1/alertSilence/:id [get]
func (receiver *alertSilenceController) QueryAlertSilence(c *gin.Context) {
	ResponseWithData(c, receiver.alertSilenceService.QuerySilence, bindTypeUri)
}

// @Summary 获取所有 AlerSilence
// @Description 获取所有 AlerSilence
// @Tags AlerSilence 管理
// @Accept json
// @Produce json
// @Param data body types.AlertSilenceListRequest true "List请求参数"
// @Success 200 {object} types.Response{data=types.AlertSilenceListResponse} "查询成功"
// @Router /api/v1/alertSilence [get]
func (receiver *alertSilenceController) ListAlertSilence(c *gin.Context) {
	ResponseWithData(c, receiver.alertSilenceService.ListSilence, bindTypeQuery)
}
