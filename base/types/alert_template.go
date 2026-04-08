package types

import "github.com/alert666/api-server/model"

type AlertTemplateCreateRequest struct {
	Name                string `json:"name" binding:"required"`
	Description         string `json:"description"`
	Template            string `json:"template" binding:"required,base64"`
	AggregationTemplate string `json:"aggregationTemplate" binding:"omitempty,base64"`
}

type AlertTemplateUpdateRequest struct {
	*IDRequest
	Template            string `json:"template" binding:"required,base64"`
	AggregationTemplate string `json:"aggregationTemplate" binding:"omitempty,base64"`
	Description         string `json:"description"`
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
