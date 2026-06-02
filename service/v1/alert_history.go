package v1

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"github.com/alert666/api-server/base/constant"
	"github.com/alert666/api-server/base/helper"
	"github.com/alert666/api-server/base/types"
	"github.com/alert666/api-server/model"
	"github.com/alert666/api-server/store"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

type AlertHistoryServicer interface {
	QueryHistory(ctx context.Context, req *types.IDRequest) (*model.AlertHistory, error)
	UpdateHistory(ctx context.Context, req *types.AlertHistoryUpdateRequest) error
	ListHistory(ctx context.Context, req *types.AlertHistoryListRequest) (*types.AlertHistoryListResponse, error)
	GetTenantFiringCounts(ctx context.Context) ([]*types.TenantCount, error)
	CacheAlertNameOptioner
}

type alertHistoryService struct {
	cacheImpl store.CacheStorer
	sf        singleflight.Group
}

func NewHistoryServicer(cache store.CacheStorer) AlertHistoryServicer {
	return &alertHistoryService{
		cacheImpl: cache,
	}
}

func NewCacheAlertNameOptioner(cache store.CacheStorer) CacheAlertNameOptioner {
	return &alertHistoryService{
		cacheImpl: cache,
	}
}

func (recevicer *alertHistoryService) QueryHistory(ctx context.Context, req *types.IDRequest) (*model.AlertHistory, error) {
	tenant, err := helper.GetTenant(ctx)
	if err != nil {
		return nil, err
	}
	return aHistoryStore.
		WithContext(ctx).
		Preload(aHistoryStore.AlertSilence).
		// Preload(aHistory.AlertChannel).
		// Preload(aHistory.AlertChannel.AlertTemplate).
		Preload(aHistoryStore.AlertSendRecord).
		Where(aHistoryStore.ID.Eq(int(req.ID))).
		Where(aHistoryStore.Cluster.Eq(tenant)).
		First()
}

func (recevicer *alertHistoryService) ListHistory(ctx context.Context, req *types.AlertHistoryListRequest) (*types.AlertHistoryListResponse, error) {
	var (
		alertAlertHistorys []*model.AlertHistory
		total              int64
		query              = aHistoryStore.WithContext(ctx)
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
		sort, ok := aHistoryStore.GetFieldByName(req.Sort)
		if !ok {
			return nil, fmt.Errorf("invalid sort field: %s", req.Sort)
		}
		query = query.Order(helper.Sort(sort, req.Direction))
	} else {
		query = query.Order(aHistoryStore.StartsAt.Desc())
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
		query = query.Where(aHistoryStore.Cluster.Eq(tenant))
	}
	if req.Status != "" {
		query = query.Where(aHistoryStore.Status.Eq(req.Status))
	}
	if req.StartsAt != nil {
		s := time.Unix(*req.StartsAt, 0)
		query = query.Where(aHistoryStore.StartsAt.Gte(s))
	}
	if req.EndsAt != nil {
		e := time.Unix(*req.EndsAt, 0)
		query = query.Where(aHistoryStore.EndsAt.Lte(e))
	}
	if req.AlertName != "" {
		query = query.Where(aHistoryStore.Alertname.Like(req.AlertName + "%"))
	}
	if req.Fingerprint != "" {
		query = query.Where(aHistoryStore.Fingerprint.Like(req.Fingerprint + "%"))
	}
	if req.Severity != "" {
		query = query.Where(aHistoryStore.Severity.Eq(req.Severity))
	}
	if req.Instance != "" {
		query = query.Where(aHistoryStore.Instance.Eq(req.Instance))
	}
	if req.AlertSendRecordId != 0 {
		query = query.Where(aHistoryStore.AlertSendRecordID.Eq(req.AlertSendRecordId))
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
	info, err := aHistoryStore.WithContext(ctx).Where(aHistoryStore.ID.Eq(int(req.ID))).Updates(model.AlertHistory{
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
	err := aHistoryStore.WithContext(ctx).
		Select(aHistoryStore.Cluster, aHistoryStore.ID.Count().As("count")).
		Where(aHistoryStore.Status.Eq(constant.AlertStatusFiring)).
		Group(aHistoryStore.Cluster).
		Scan(&results)

	if err != nil {
		return nil, err
	}

	return results, nil
}

type CacheAlertNameOptioner interface {
	GetAlertNameOptions(ctx context.Context) ([]types.Option, error)
	CacheAlertNameOptions()
}

func (receiver *alertHistoryService) GetAlertNameOptions(ctx context.Context) ([]types.Option, error) {
	var options []types.Option
	exits, err := receiver.cacheImpl.GetObject(ctx, store.AlertNameType, constant.OptionsCacheKey, &options)
	if err != nil {
		return nil, err
	}

	if exits {
		return options, nil
	}

	_, err, _ = receiver.sf.Do(constant.OptionsCacheKey, func() (interface{}, error) {
		receiver.CacheAlertNameOptions()
		return nil, nil
	})
	if err != nil {
		return nil, fmt.Errorf("缓存 alertNameOptions 失败, %w", err)
	}

	receiver.CacheAlertNameOptions()
	exits, err = receiver.cacheImpl.GetObject(ctx, store.AlertNameType, constant.OptionsCacheKey, &options)
	if err != nil {
		return nil, err
	}

	return options, nil
}

func (receiver *alertHistoryService) CacheAlertNameOptions() {
	start := time.Now()
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			zap.L().Error("CacheAlertNameOptions panic recovered",
				zap.Any("panic", r),
				zap.String("stack", string(stack)),
			)
			return
		}
		elapsed := time.Since(start).Milliseconds()
		zap.L().Info("CacheAlertNameOptions 执行结束",
			zap.Int64("duration_ms", elapsed),
		)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 58*time.Second)
	defer cancel()

	ok, err := receiver.cacheImpl.SetNX(ctx, store.LockType, constant.AlertNamesOptionsLockKey, time.Now().Unix(), 58*time.Second)
	if err != nil {
		zap.L().Error("[定时任务] CacheAlertNameOptions Redis 分布式锁异常", zap.Error(err))
		return
	}
	defer receiver.cacheImpl.DelKey(ctx, store.LockType, constant.AlertNamesOptionsLockKey)

	if !ok {
		zap.L().Debug("[定时任务] CacheAlertNameOptions 缓存 AlertNames 任务正在其他节点运行，本次跳过")
		return
	}

	zap.L().Debug("[定时任务] CacheAlertNameOptions 成功获取锁，开始缓存 AlertNames")

	options, err := receiver.cacheAlertNameOptions(ctx)
	if err != nil {
		zap.L().Error("[定时任务] 获取 alertName Options 失败", zap.Error(err))
	}

	if len(options) == 0 {
		return
	}

	if err = receiver.cacheImpl.SetObject(ctx, store.AlertNameType, constant.OptionsCacheKey, options, store.NeverExpires); err != nil {
		zap.L().Error("[定时任务] 设置 alertName Options 失败", zap.Error(err))
	}
}

func (receiver *alertHistoryService) cacheAlertNameOptions(ctx context.Context) ([]types.Option, error) {
	var options []types.Option
	err := store.AlertHistory.WithContext(ctx).UnderlyingDB().
		Model(&model.AlertHistory{}).
		Select("distinct alertname as label, alertname as value").
		Scan(&options).Error

	if err != nil {
		return nil, err
	}

	return options, nil
}
