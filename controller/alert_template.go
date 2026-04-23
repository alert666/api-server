package controller

import (
	"github.com/alert666/api-server/base/bind"
	v1 "github.com/alert666/api-server/service/v1"
	"github.com/gin-gonic/gin"
)

type AlertTemplateController interface {
	CreateAlertTemplate(c *gin.Context)
	UpdateAlertTemplate(c *gin.Context)
	DeleteAlertTemplate(c *gin.Context)
	QueryAlertTemplate(c *gin.Context)
	ListAlertTemplate(c *gin.Context)
}

type alertTemplateController struct {
	alertTemplateService v1.AlertTemplateServicer
}

func NewAlertTemplateController(alertTemplateService v1.AlertTemplateServicer) AlertTemplateController {
	return &alertTemplateController{
		alertTemplateService: alertTemplateService,
	}
}

// CreateApi 创建 AlerTemplate
// @Summary 创建 AlerTemplate
// @Description 创建 AlerTemplate
// @Tags AAlerTemplate 管理
// @Accept json
// @Produce json
// @Param data body types.AlertTemplateCreateRequest true "创建请求参数"
// @Success 200 {object} types.Response "创建成功"
// @Router /api/v1/alertTemplate [post]
func (receiver *alertTemplateController) CreateAlertTemplate(c *gin.Context) {
	bind.ResponseOnlySuccess(c, receiver.alertTemplateService.CreateAlerTemplate, bind.BindTypeJson)
}

// UpdateApi 更新 AlerTemplate
// @Summary 更新 AlerTemplate
// @Description 更新 AlerTemplate
// @Tags AlerTemplate 管理
// @Accept json
// @Produce json
// @Param data body types.AlertTemplateUpdateRequest true "更新请求参数"
// @Success 200 {object} types.Response "更新成功"
// @Router /api/v1/alertTemplate/:id [put]
func (receiver *alertTemplateController) UpdateAlertTemplate(c *gin.Context) {
	bind.ResponseOnlySuccess(c, receiver.alertTemplateService.UpdateTemplate, bind.BindTypeJson, bind.BindTypeUri)
}

// DeleteApi 删除 AlerTemplate
// @Summary 删除 AlerTemplate
// @Description 删除 AlerTemplate
// @Tags AlerTemplate 管理
// @Accept json
// @Produce json
// @Param data body types.IDRequest true "删除请求参数"
// @Success 200 {object} types.Response "删除成功"
// @Router /api/v1/AlertTemplate/:id [delete]
func (receiver *alertTemplateController) DeleteAlertTemplate(c *gin.Context) {
	bind.ResponseOnlySuccess(c, receiver.alertTemplateService.DeleteTemplate, bind.BindTypeUri)
}

// QueryApi 查询 AlerTemplate
// @Summary 查询 AlerTemplate
// @Description 查询 AlerTemplate
// @Tags AlerTemplate 管理
// @Accept json
// @Produce json
// @Param data body types.IDRequest true "查询请求参数"
// @Success 200 {object} types.Response{data=model.AlertTemplate} "查询成功"
// @Router /api/v1/AlertTemplate/:id [get]
func (receiver *alertTemplateController) QueryAlertTemplate(c *gin.Context) {
	bind.ResponseWithData(c, receiver.alertTemplateService.QueryTemplate, bind.BindTypeUri)
}

// @Summary 获取所有 AlerTemplate
// @Description 获取所有 AlerTemplate
// @Tags AlerTemplate 管理
// @Accept json
// @Produce json
// @Success 200 {object} types.Response{data=types.AlertTemplateListResponse} "查询成功"
// @Router /api/v1/AlertTemplate [get]
func (receiver *alertTemplateController) ListAlertTemplate(c *gin.Context) {
	bind.ResponseWithData(c, receiver.alertTemplateService.ListTemplate, bind.BindTypeUri, bind.BindTypeQuery)
}
