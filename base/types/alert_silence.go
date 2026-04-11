package types

import (
	"encoding/json"
	"time"

	"github.com/alert666/api-server/model"
)

type AlertSilenceCreateRequest struct {
	Cluster     string          `json:"cluster" binding:"required,max=64"`
	Type        int             `json:"type"  binding:"required,oneof=1 2"`
	Status      *int            `json:"status" binding:"required,oneof=0 1"`
	StartsAt    time.Time       `json:"startsAt" binding:"required"`
	Fingerprint string          `json:"fingerprint" binding:"required_without=Matchers"`
	EndsAt      time.Time       `json:"endsAt" binding:"required"`
	Matchers    []model.Matcher `json:"matchers" binding:"required_without=Fingerprint"`
	CreatedBy   string          `json:"createdBy" binding:"required"`
	Comment     string          `json:"comment" binding:"required,max=255"`
}

func (receiver *AlertSilenceCreateRequest) TOMolelAlertSilence() (*model.AlertSilence, error) {
	mBytes, err := json.Marshal(receiver.Matchers)
	if err != nil {
		return nil, err
	}

	return &model.AlertSilence{
		Cluster:     receiver.Cluster,
		Type:        receiver.Type,
		Status:      receiver.Status,
		StartsAt:    receiver.StartsAt,
		EndsAt:      receiver.EndsAt,
		CreatedBy:   receiver.CreatedBy,
		Comment:     receiver.Comment,
		Fingerprint: receiver.Fingerprint,
		Matchers:    mBytes,
	}, nil
}

type AlertSilenceListRequest struct {
	*Pagination
	Cluster   string          `form:"cluster" binding:"omitempty,max=64"`
	Status    *int            `form:"status" binding:"required,oneof=0 1"`
	StartsAt  time.Time       `form:"startsAt" binding:"required"`
	EndsAt    time.Time       `form:"endsAt" binding:"required,gtfield=StartsAt"`
	Matchers  []model.Matcher `form:"matchers" binding:"omitempty,gt=0"`
	CreatedBy string          `form:"createdBy" binding:"omitempty"`
	Sort      string          `form:"sort" binding:"omitempty,oneof=id name created_at updated_at"`
	Direction string          `form:"direction" binding:"omitempty,oneof=asc desc"`
}

type AlertSilenceListResponse struct {
	*ListResponse
	List []*model.AlertSilence `json:"list"`
}

func NewAlertSilenceListResponse(alertSilences []*model.AlertSilence, total int64, pageSize, page int) *AlertSilenceListResponse {
	return &AlertSilenceListResponse{
		ListResponse: &ListResponse{
			Total: total,
			Pagination: &Pagination{
				Page:     page,
				PageSize: pageSize,
			},
		},
		List: alertSilences,
	}
}
