package v1

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alert666/api-server/base/constant"
	"github.com/alert666/api-server/base/helper"
	"github.com/alert666/api-server/base/types"
	"github.com/alert666/api-server/model"
	"github.com/alert666/api-server/store"
)

type AlertHistoryServicer interface {
	QueryHistory(ctx context.Context, req *types.IDRequest) (*model.AlertHistory, error)
	UpdateHistory(ctx context.Context, req *types.AlertHistoryUpdateRequest) error
	ListHistory(ctx context.Context, req *types.AlertHistoryListRequest) (*types.AlertHistoryListResponse, error)
	GetTenantFiringCounts(ctx context.Context) ([]*types.TenantCount, error)
}

type alertHistoryService struct {
	cache store.CacheStorer
}

func NewHistoryServicer(cache store.CacheStorer) AlertHistoryServicer {
	return &alertHistoryService{}
}

func (recevicer *alertHistoryService) QueryHistory(ctx context.Context, req *types.IDRequest) (*model.AlertHistory, error) {
	tenant, err := helper.GetTenant(ctx)
	if err != nil {
		return nil, err
	}
	return aHistory.WithContext(ctx).Where(aHistory.ID.Eq(int(req.ID))).Where(aHistory.Cluster.Eq(tenant)).First()
}

func (recevicer *alertHistoryService) ListHistory(ctx context.Context, req *types.AlertHistoryListRequest) (*types.AlertHistoryListResponse, error) {
	var (
		alertAlertHistorys []*model.AlertHistory
		total              int64
		query              = aHistory.WithContext(ctx)
		err                error
	)
	tenant, err := helper.GetTenant(ctx)
	if err != nil {
		return nil, err
	}

	query = recevicer.buildHistoryFilter(tenant, query, req)

	if total, err = query.Count(); err != nil {
		return nil, err
	}

	if req.Sort != "" && req.Direction != "" {
		sort, ok := aHistory.GetFieldByName(req.Sort)
		if !ok {
			return nil, fmt.Errorf("invalid sort field: %s", req.Sort)
		}
		query = query.Order(helper.Sort(sort, req.Direction))
	} else {
		query = query.Order(aHistory.StartsAt.Desc())
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
func (s *alertHistoryService) buildHistoryFilter(tenant string, query store.IAlertHistoryDo, req *types.AlertHistoryListRequest) store.IAlertHistoryDo {
	if tenant != "" {
		query = query.Where(aHistory.Cluster.Eq(tenant))
	}
	if req.Status != "" {
		query = query.Where(aHistory.Status.Eq(req.Status))
	}
	if req.StartsAt != nil {
		s := time.Unix(*req.StartsAt, 0)
		query = query.Where(aHistory.StartsAt.Gte(s))
	}
	if req.EndsAt != nil {
		e := time.Unix(*req.EndsAt, 0)
		query = query.Where(aHistory.EndsAt.Lte(e))
	}
	if req.AlertName != "" {
		query = query.Where(aHistory.Alertname.Like(req.AlertName + "%"))
	}
	if req.Fingerprint != "" {
		query = query.Where(aHistory.Fingerprint.Like(req.Fingerprint + "%"))
	}
	if req.Severity != "" {
		query = query.Where(aHistory.Severity.Eq(req.Severity))
	}
	if req.Instance != "" {
		query = query.Where(aHistory.Instance.Eq(req.Instance))
	}
	if req.AlertSendRecordId != 0 {
		query = query.Where(aHistory.AlertSendRecordID.Eq(req.AlertSendRecordId))
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

func (receiver *alertHistoryService) UpdateHistory(ctx context.Context, req *types.AlertHistoryUpdateRequest) error {
	now := time.Now()
	info, err := aHistory.WithContext(ctx).Where(aHistory.ID.Eq(int(req.ID))).Updates(model.AlertHistory{
		Status: req.Status,
		EndsAt: &now,
	})
	if err != nil {
		return fmt.Errorf("更新 alertHistory 失败, %w", err)
	}

	if info.RowsAffected == 0 {
		return fmt.Errorf("更新失败：告警记录(ID:%d)不存在", req.ID)
	}
	return nil
}

func (receiver *alertHistoryService) GetTenantFiringCounts(ctx context.Context) ([]*types.TenantCount, error) {
	var results []*types.TenantCount

	// SQL: SELECT cluster, count(*) as count FROM alert_historys WHERE status = 'firing' GROUP BY cluster
	err := aHistory.WithContext(ctx).
		Select(aHistory.Cluster, aHistory.ID.Count().As("count")).
		Where(aHistory.Status.Eq(constant.AlertStatusFiring)).
		Group(aHistory.Cluster).
		Scan(&results)

	if err != nil {
		return nil, err
	}

	return results, nil
}
