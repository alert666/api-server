package v1

import (
	"context"
	"fmt"
	"strings"

	"github.com/qinquanliuxiang666/alertmanager/base/helper"
	"github.com/qinquanliuxiang666/alertmanager/base/types"
	"github.com/qinquanliuxiang666/alertmanager/model"
	"github.com/qinquanliuxiang666/alertmanager/store"
)

type AlertHistoryServicer interface {
	QueryHistory(ctx context.Context, req *types.IDRequest) (*model.AlertHistory, error)
	ListHistory(ctx context.Context, req *types.AlertHistoryListRequest) (*types.AlertHistoryListResponse, error)
}

type alertHistoryService struct {
	cache store.CacheStorer
}

func NewHistoryServicer(cache store.CacheStorer) AlertHistoryServicer {
	return &alertHistoryService{}
}

func (recevicer *alertHistoryService) QueryHistory(ctx context.Context, req *types.IDRequest) (*model.AlertHistory, error) {
	return al.WithContext(ctx).Where(al.ID.Eq(int(req.ID))).First()
}

func (recevicer *alertHistoryService) ListHistory(ctx context.Context, req *types.AlertHistoryListRequest) (*types.AlertHistoryListResponse, error) {
	var (
		alertAlertHistorys []*model.AlertHistory
		total              int64
		query              = al.WithContext(ctx)
		err                error
	)
	query = recevicer.buildHistoryFilter(query, req)

	if total, err = query.Count(); err != nil {
		return nil, err
	}

	if req.Sort != "" && req.Direction != "" {
		sort, ok := al.GetFieldByName(req.Sort)
		if !ok {
			return nil, fmt.Errorf("invalid sort field: %s", req.Sort)
		}
		query = query.Order(helper.Sort(sort, req.Direction))
	}

	if req.PageSize == 0 || req.Page == 0 {
		return nil, fmt.Errorf("page and pageSize must be greater than 0")
	}

	if alertAlertHistorys, err = query.Limit(req.PageSize).Offset((req.Page - 1) * req.PageSize).Find(); err != nil {
		return nil, err
	}
	return types.NewAlertHistoryListResponse(alertAlertHistorys, total, req.PageSize, req.Page), nil
}

// 提取过滤逻辑，提高可读性
func (s *alertHistoryService) buildHistoryFilter(query store.IAlertHistoryDo, req *types.AlertHistoryListRequest) store.IAlertHistoryDo {
	if req.Cluster != "" {
		query = query.Where(al.Cluster.Eq(req.Cluster))
	}
	if req.StartsAt != nil {
		query = query.Where(al.StartsAt.Gt(*req.StartsAt))
	}
	if req.EndsAt != nil {
		query = query.Where(al.EndsAt.Gt(*req.EndsAt))
	}
	if req.AlertName != "" {
		query = query.Where(al.Alertname.Like(req.AlertName + "%"))
	}
	if req.Fingerprint != "" {
		query = query.Where(al.Fingerprint.Like(req.Fingerprint + "%"))
	}
	if req.Severity != "" {
		query = query.Where(al.Severity.Eq(req.Severity))
	}
	if req.Instance != "" {
		query = query.Where(al.Instance.Eq(req.Instance))
	}

	if len(req.Labels) > 0 {
		db := query.UnderlyingDB()
		for _, item := range req.Labels {
			parts := strings.SplitN(item, "=", 2)
			if len(parts) == 2 {
				key := parts[0]
				value := parts[1]

				jsonPath := fmt.Sprintf(`$."%s"`, key)
				db = db.Where("JSON_UNQUOTE(JSON_EXTRACT(`labels`, ?)) = ?", jsonPath, value)
			}
		}
		query.ReplaceDB(db)
	}
	return query
}
