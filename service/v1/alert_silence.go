package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/alert666/api-server/base/constant"
	"github.com/alert666/api-server/base/log"
	"github.com/alert666/api-server/base/helper"
	"github.com/alert666/api-server/pkg/jwt"

	"github.com/alert666/api-server/base/types"
	"github.com/alert666/api-server/model"
	"github.com/alert666/api-server/store"
	"go.uber.org/zap"
)

type AlertSilenceServicer interface {
	CreateSilence(ctx context.Context, req *types.AlertSilenceCreateRequest) error
	DeleteSilence(ctx context.Context, req *types.IDRequest) error
	QuerySilence(ctx context.Context, req *types.IDRequest) (*model.AlertSilence, error)
	ListSilence(ctx context.Context, req *types.AlertSilenceListRequest) (*types.AlertSilenceListResponse, error)
	GetTenantSilenceCounts(ctx context.Context) ([]*types.TenantCount, error)
}

type alertSilenceService struct {
	jwtImpl   jwt.JwtInterface
	cacheImpl store.CacheStorer
}

func NewAlertSilenceServicer(cache store.CacheStorer, jwt jwt.JwtInterface) AlertSilenceServicer {
	return &alertSilenceService{
		cacheImpl: cache,
		jwtImpl:   jwt,
	}
}

func (recevicer *alertSilenceService) CreateSilence(ctx context.Context, req *types.AlertSilenceCreateRequest) error {
	var count int64
	tenant, err := helper.GetTenant(ctx)
	if err != nil {
		return err
	}

	mc, err := recevicer.jwtImpl.GetUser(ctx)
	if err != nil {
		return err
	}

	obj, err := req.TOMolelAlertSilence()
	if err != nil {
		return fmt.Errorf("转换对象失败, %s", err)
	}
	obj.Cluster = tenant
	obj.CreatedBy = mc.UserName

	err = store.Q.Transaction(func(tx *store.Query) error {
		if req.Type != model.SilenceTypeIdentity {
			tx.AlertSilence.WithContext(ctx).UnderlyingDB().Where(
				"cluster = ? AND matchers = ? AND status = 1 AND ends_at > ?",
				obj.Cluster, obj.Matchers, time.Now(),
			).Count(&count)
			if count > 0 {
				return fmt.Errorf("已存在相同的活跃静默规则")
			}
		}

		if err := tx.AlertSilence.WithContext(ctx).Create(obj); err != nil {
			return err
		}

		if req.Type == model.SilenceTypeIdentity {
			if _, err := tx.
				AlertHistory.
				WithContext(ctx).
				Where(tx.AlertHistory.Cluster.Eq(tenant)).
				Where(tx.AlertHistory.Fingerprint.Eq(req.Fingerprint)).
				Where(tx.AlertHistory.Status.Eq(constant.AlertStatusFiring)).
				Updates(model.AlertHistory{
					IsSilenced:     true,
					AlertSilenceID: &obj.ID,
				}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := recevicer.cacheImpl.DelKey(ctx, store.AlertSilenceType, tenant); err != nil {
		log.WithRequestID(ctx).Error("静默规则缓存清理失败", zap.Error(err))
	}
	return nil
}

func (recevicer *alertSilenceService) DeleteSilence(ctx context.Context, req *types.IDRequest) error {
	tenant, err := helper.GetTenant(ctx)
	if err != nil {
		return err
	}

	// 直接执行带条件的删除
	info, err := aSilenceStore.WithContext(ctx).
		Where(aSilenceStore.ID.Eq(int(req.ID))).
		Where(aSilenceStore.Cluster.Eq(tenant)).
		Delete()

	if err != nil {
		return err
	}

	// 如果没有行受影响，说明 ID 不存在或者不属于该租户
	if info.RowsAffected == 0 {
		return fmt.Errorf("ID %d alertSilence 不存在或者 tenant 不匹配", req.ID)
	}
	if err := recevicer.cacheImpl.DelKey(ctx, store.AlertSilenceType, tenant); err != nil {
		log.WithRequestID(ctx).Error("静默规则缓存清理失败", zap.Error(err))
	}
	return nil
}

func (recevicer *alertSilenceService) QuerySilence(ctx context.Context, req *types.IDRequest) (*model.AlertSilence, error) {
	tenant, err := helper.GetTenant(ctx)
	if err != nil {
		return nil, err
	}
	return aSilenceStore.WithContext(ctx).Where(aSilenceStore.ID.Eq(int(req.ID))).Where(aSilenceStore.Cluster.Eq(tenant)).First()
}

func (recevicer *alertSilenceService) ListSilence(ctx context.Context, req *types.AlertSilenceListRequest) (*types.AlertSilenceListResponse, error) {
	var (
		AlertSilences []*model.AlertSilence
		total         int64
		query         = aSilenceStore.WithContext(ctx).UnderlyingDB()
	)
	tenant, err := helper.GetTenant(ctx)
	if err != nil {
		return nil, err
	}
	if tenant != "" {
		query = query.Where(aSilenceStore.Cluster.Eq(tenant))
	}

	if req.Status != nil {
		query = query.Where(aSilenceStore.Status.Eq(*req.Status))
	}

	if req.StartsAt != nil && req.EndsAt != nil {
		if *req.EndsAt <= *req.StartsAt {
			return nil, fmt.Errorf("endsAt 必须大于 startsAt")
		}
	}

	if req.StartsAt != nil {
		startsAt := time.Unix(*req.StartsAt, 0)
		query = query.Where(aSilenceStore.StartsAt.Gte(startsAt))
	}

	if req.EndsAt != nil {
		endsAt := time.Unix(*req.EndsAt, 0)
		query = query.Where(aSilenceStore.EndsAt.Lte(endsAt))
	}

	if len(req.Matchers) > 0 {
		mBytes, err := json.Marshal(req.Matchers)
		if err != nil {
			return nil, err
		}
		query = query.Where("JSON_CONTAINS(matchers, ?)", string(mBytes))
	}

	if req.CreatedBy != "" {
		query = query.Where(aSilenceStore.CreatedBy.Eq(req.CreatedBy))
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	if req.Sort != "" && req.Direction != "" {
		order := fmt.Sprintf("%s %s", req.Sort, req.Direction)
		query = query.Order(order)
	} else {
		query = query.Order(aSilenceStore.CreatedAt.Desc())
	}

	if req.PageSize == 0 || req.Page == 0 {
		return nil, fmt.Errorf("pageSize 和 page 不能为0")
	}

	if err := query.Limit(req.PageSize).Offset((req.Page - 1) * req.PageSize).Find(&AlertSilences).Error; err != nil {
		return nil, err
	}

	return types.NewAlertSilenceListResponse(AlertSilences, total, req.PageSize, req.Page), nil
}

func (recevicer *alertSilenceService) GetTenantSilenceCounts(ctx context.Context) ([]*types.TenantCount, error) {
	var results []*types.TenantCount

	// SQL: SELECT cluster, count(*) as count FROM alert_silences WHERE status = 1 GROUP BY cluster
	err := aSilenceStore.WithContext(ctx).
		Select(aSilenceStore.Cluster, aSilenceStore.ID.Count().As("count")).
		Where(aSilenceStore.Status.Eq(model.SilenceEnabled)).
		Group(aSilenceStore.Cluster).
		Scan(&results)

	if err != nil {
		return nil, err
	}

	return results, nil
}

type CleanExpiredSilencer interface {
	CleanExpiredSilencesTask()
}

type CleanExpiredSilence struct {
	cacheImpl store.CacheStorer
}

func NewCleanExpiredSilencer(cache store.CacheStorer) CleanExpiredSilencer {
	return &CleanExpiredSilence{
		cacheImpl: cache,
	}
}

// CleanExpiredSilencesTask 定时任务：清理过期的静默规则
func (recevicer *CleanExpiredSilence) CleanExpiredSilencesTask() {
	start := time.Now()
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			zap.L().Error("CleanExpiredSilencesTask panic recovered",
				zap.Any("panic", r),
				zap.String("stack", string(stack)),
			)
			return
		}
		elapsed := time.Since(start).Milliseconds()
		zap.L().Debug("CleanExpiredSilencesTask 执行结束",
			zap.Int64("duration_ms", elapsed),
		)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 58*time.Second)
	defer cancel()

	ok, err := recevicer.cacheImpl.SetNX(ctx, store.LockType, constant.AlertCleanExpiredSilencesLockKey, time.Now().Unix(), 58*time.Second)
	if err != nil {
		zap.L().Error("[定时任务] CleanExpiredSilencesTask Redis 分布式锁异常", zap.Error(err))
		return
	}
	defer recevicer.cacheImpl.DelKey(ctx, store.LockType, constant.AlertCleanExpiredSilencesLockKey)

	if !ok {
		zap.L().Debug("[定时任务] CleanExpiredSilencesTask 清理过期静默规则任务正在其他节点运行，本次跳过")
		return
	}

	zap.L().Debug("[定时任务] 成功获取锁，开始清理过期静默规则")

	now := time.Now()
	// 将已过期的规则状态从 1 改为 0 ---
	// 逻辑：如果结束时间早于现在，且状态还是“启用”，则更新为“禁用/过期”

	// 先查询受影响的集群，用于清理缓存
	var expiredClusters []string
	if err := aSilenceStore.WithContext(ctx).
		Select(aSilenceStore.Cluster).
		Where(aSilenceStore.Status.Eq(model.SilenceEnabled)).
		Where(aSilenceStore.EndsAt.Lt(now)).
		Distinct().
		Scan(&expiredClusters); err != nil {
		zap.L().Error("[定时任务] 查询过期静默规则集群失败", zap.Error(err))
	}

	info, err := aSilenceStore.WithContext(ctx).
		Where(aSilenceStore.Status.Eq(model.SilenceEnabled)).
		Where(aSilenceStore.EndsAt.Lt(now)).
		Update(aSilenceStore.Status, model.SilenceExpired)

	if err != nil {
		zap.L().Error("[定时任务] 更新过期静默规则状态失败", zap.Error(err))
	} else if info.RowsAffected > 0 {
		zap.L().Debug("[定时任务] 成功将过期静默规则置为失效", zap.Int64("count", info.RowsAffected))
		for _, cluster := range expiredClusters {
			if err := recevicer.cacheImpl.DelKey(ctx, store.AlertSilenceType, cluster); err != nil {
				zap.L().Error("[定时任务] 清理过期静默规则缓存失败", zap.Error(err))
			}
		}
	}

	for _, cluster := range expiredClusters {
		if err := recevicer.cacheImpl.DelKey(ctx, store.AlertSilenceType, cluster); err != nil {
			zap.L().Error("[定时任务] 清理过期静默规则缓存失败", zap.Error(err))
		}
	}
}
