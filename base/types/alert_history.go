package types

import (
	"github.com/alert666/api-server/model"
)

type AlertHistoryUpdateRequest struct {
	*IDRequest
	Status string `json:"status" binding:"required,eq=resolved"`
}

type AlertHistoryListRequest struct {
	*Pagination
	Fingerprint       string   `form:"fingerprint"`
	AlertName         string   `form:"alertName"`
	Status            string   `form:"status" binding:"omitempty,oneof=resolved firing"`
	Severity          string   `form:"severity"`
	Instance          string   `form:"instance"`
	StartsAt          *int64   `form:"startsAt"`
	EndsAt            *int64   `form:"endsAt"`
	Labels            []string `form:"labels"`
	AlertSendRecordId int      `form:"alertSendRecordId"`
	Sort              string   `form:"sort" binding:"omitempty,oneof=id alertname fingerprint starts_at ends_at severity instance"`
	Direction         string   `form:"direction" binding:"omitempty,oneof=asc desc"`
}

type AlertHistoryListResponse struct {
	*ListResponse
	List []*model.AlertHistory `json:"list"`
}

func NewAlertHistoryListResponse(alertHistorys []*model.AlertHistory, total int64, pageSize, page int) *AlertHistoryListResponse {
	return &AlertHistoryListResponse{
		ListResponse: &ListResponse{
			Total: total,
			Pagination: &Pagination{
				Page:     page,
				PageSize: pageSize,
			},
		},
		List: alertHistorys,
	}
}
