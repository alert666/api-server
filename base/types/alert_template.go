package types

import "github.com/alert666/api-server/model"

type AlertTemplateCreateRequest struct {
	Name                string `json:"name" binding:"required"`
	AlertChannelID      int    `json:"alertChannelID"`
	ReceiveIdType       string `json:"receiveIdType" binding:"required,oneof=open_id user_id email chat_id"`
	ReceiveId           []string `json:"receiveId" binding:"required"`
	Description         string `json:"description"`
	Template            string `json:"template" binding:"required,base64"`
	AggregationTemplate string `json:"aggregationTemplate" binding:"omitempty,base64"`
}

type AlertTemplateUpdateRequest struct {
	*IDRequest
	AlertChannelID      int    `json:"alertChannelID"`
	ReceiveIdType       string `json:"receiveIdType" binding:"required,oneof=open_id user_id email chat_id"`
	ReceiveId           []string `json:"receiveId" binding:"required"`
	Template            string `json:"template" binding:"required,base64"`
	AggregationTemplate string `json:"aggregationTemplate" binding:"omitempty,base64"`
	Description         string `json:"description"`
}

type AlertTemplateCopyRequest struct {
	*IDRequest
	Name string `json:"name" binding:"required"`
}

type AlertTemplateListRequest struct {
	*Pagination
	Name      string `form:"name"`
	Sort      string `form:"sort" binding:"omitempty,oneof=id name created_at updated_at"`
	Direction string `form:"direction" binding:"omitempty,oneof=asc desc"`
}

type AlertTemplateListResponse struct {
	*ListResponse
	List []*model.AlertTemplate `json:"list"`
}

func NewAlertTemplateListResponse(alertTemplates []*model.AlertTemplate, total int64, pageSize, page int) *AlertTemplateListResponse {
	return &AlertTemplateListResponse{
		ListResponse: &ListResponse{
			Total: total,
			Pagination: &Pagination{
				Page:     page,
				PageSize: pageSize,
			},
		},
		List: alertTemplates,
	}
}
