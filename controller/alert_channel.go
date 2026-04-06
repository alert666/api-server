package controller

import (
	"github.com/gin-gonic/gin"
	v1 "github.com/qinquanliuxiang666/alertmanager/service/v1"
)

type AlertChannelController interface {
	CreateAlertChannel(c *gin.Context)
	UpdateAlertChannel(c *gin.Context)
	DeleteAlertChannel(c *gin.Context)
	QueryAlertChannel(c *gin.Context)
	ListAlertChannel(c *gin.Context)
}

type alertChannelController struct {
	alertChannelService v1.AlertChannelServicer
}

func NewAlertChannelController(alertChannelService v1.AlertChannelServicer) AlertChannelController {
	return &alertChannelController{
		alertChannelService: alertChannelService,
	}
}

// CreateApi 创建 AlerChannel
// @Summary 创建 AlerChannel
// @Description 创建 AlerChannel
// @Tags AAlerChannel 管理
// @Accept json
// @Produce json
// @Param data body types.AlertChannelCreateRequest true "创建请求参数"
// @Success 200 {object} types.Response "创建成功"
// @Router /api/v1/alertChannel [post]
func (receiver *alertChannelController) CreateAlertChannel(c *gin.Context) {
	ResponseOnlySuccess(c, receiver.alertChannelService.CreateAlerChannel, bindTypeJson)
}

// UpdateApi 更新 AlerChannel
// @Summary 更新 AlerChannel
// @Description 更新 AlerChannel
// @Tags AlerChannel 管理
// @Accept json
// @Produce json
// @Param data body types.AlertChannelUpdateRequest true "更新请求参数"
// @Success 200 {object} types.Response "更新成功"
// @Router /api/v1/alertChannel/:id [put]
func (receiver *alertChannelController) UpdateAlertChannel(c *gin.Context) {
	ResponseOnlySuccess(c, receiver.alertChannelService.UpdateChannel, bindTypeJson, bindTypeUri)
}

// DeleteApi 删除 AlerChannel
// @Summary 删除 AlerChannel
// @Description 删除 AlerChannel
// @Tags AlerChannel 管理
// @Accept json
// @Produce json
// @Param data body types.IDRequest true "删除请求参数"
// @Success 200 {object} types.Response "删除成功"
// @Router /api/v1/alertChannel/:id [delete]
func (receiver *alertChannelController) DeleteAlertChannel(c *gin.Context) {
	ResponseOnlySuccess(c, receiver.alertChannelService.DeleteChannel, bindTypeUri)
}

// QueryApi 查询 AlerChannel
// @Summary 查询 AlerChannel
// @Description 查询 AlerChannel
// @Tags AlerChannel 管理
// @Accept json
// @Produce json
// @Param data body types.IDRequest true "查询请求参数"
// @Success 200 {object} types.Response{data=model.AlertChannel} "查询成功"
// @Router /api/v1/alertChannel/:id [get]
func (receiver *alertChannelController) QueryAlertChannel(c *gin.Context) {
	ResponseWithData(c, receiver.alertChannelService.QueryChannel, bindTypeUri)
}

// @Summary 获取所有 AlerChannel
// @Description 获取所有 AlerChannel
// @Tags AlerChannel 管理
// @Accept json
// @Produce json
// @Param data body types.AlertChannelListRequest true "查询请求参数"
// @Success 200 {object} types.Response{data=types.AlertChannelListResponse} "查询成功"
// @Router /api/v1/alertChannel [get]
func (receiver *alertChannelController) ListAlertChannel(c *gin.Context) {
	ResponseWithData(c, receiver.alertChannelService.ListChannel, bindTypeUri, bindTypeQuery)
}
