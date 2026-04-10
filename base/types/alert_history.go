package types

import (
	"time"

	"github.com/alert666/api-server/model"
)

type AlertHistoryListRequest struct {
	*Pagination
	Cluster     string     `form:"cluster"`
	Fingerprint string     `form:"fingerprint"`
	AlertName   string     `form:"alertName"`
	Status      string     `form:"status" binding:"oneof=resolved firing"`
	Severity    string     `form:"severity"`
	Instance    string     `form:"instance"`
	StartsAt    *time.Time `form:"startsAt"`
	EndsAt      *time.Time `form:"endsAt"`
	Labels      []string   `form:"labels"`
	Sort        string     `form:"sort" binding:"omitempty,oneof=alertname fingerprint starts_at ends_at severity instance"`
	Direction   string     `form:"direction" binding:"omitempty,oneof=asc desc"`
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
