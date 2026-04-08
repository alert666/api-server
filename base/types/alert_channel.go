package types

import (
	"github.com/alert666/api-server/model"
)

type AlertChannelCreateRequest struct {
	Name              string         `json:"name" binding:"required,max=15"`
	Type              string         `json:"type" binding:"required,oneof=feishuApp feishuBoot webhook"`
	Status            *int           `json:"status" binding:"required,oneof=0 1"`
	AggregationStatus *int           `json:"aggregationStatus" binding:"oneof=0 1"`
	Config            map[string]any `json:"config" binding:"required"`
	Description       string         `json:"description"`
}

type AlertChannelUpdateRequest struct {
	*IDRequest
	Type              string         `json:"type" binding:"required,oneof=feishuApp feishuBoot webhook"`
	Status            *int           `json:"status" binding:"required,oneof=0 1"`
	AggregationStatus *int           `json:"aggregationStatus" binding:"required,oneof=0 1"`
	Config            map[string]any `json:"config" binding:"required"`
	Description       string         `json:"description"`
	TemplateID        int            `json:"templateID"`
}

type AlertChannelListRequest struct {
	*Pagination
	Name      string `form:"name"`
	Type      string `form:"type" binding:"omitempty,oneof=feishuApp feishuBoot webhook"`
	Sort      string `form:"sort" binding:"omitempty,oneof=id name created_at updated_at"`
	Direction string `form:"direction" binding:"omitempty,oneof=asc desc"`
}

type AlertChannelListResponse struct {
	*ListResponse
	List []*model.AlertChannel `json:"list"`
}

func NewAlertChannelListResponse(alertChannels []*model.AlertChannel, total int64, pageSize, page int) *AlertChannelListResponse {
	return &AlertChannelListResponse{
		ListResponse: &ListResponse{
			Total: total,
			Pagination: &Pagination{
				Page:     page,
				PageSize: pageSize,
			},
		},
		List: alertChannels,
	}
}
