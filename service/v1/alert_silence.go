package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/alert666/api-server/base/constant"

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
}

type alertSilenceService struct{}

func NewAlertSilenceServicer() AlertSilenceServicer {
	return &alertSilenceService{}
}

func (recevicer *alertSilenceService) CreateSilence(ctx context.Context, req *types.AlertSilenceCreateRequest) error {
	obj, err := req.TOMolelAlertSilence()
	if err != nil {
		return fmt.Errorf("转换对象失败, %s", err)
	}

	var count int64
	aSilence.WithContext(ctx).UnderlyingDB().Where(
		"cluster = ? AND matchers = ? AND status = 1 AND ends_at > ?",
		obj.Cluster, obj.Matchers, time.Now(),
	).Count(&count)

	if count > 0 {
		return fmt.Errorf("已存在相同的活跃静默规则")
	}

	return aSilence.WithContext(ctx).Create(obj)
}

func (recevicer *alertSilenceService) DeleteSilence(ctx context.Context, req *types.IDRequest) error {
	obj, err := aSilence.WithContext(ctx).Where(aSilence.ID.Eq(int(req.ID))).First()
	if err != nil {
		return err
	}

	if _, err := aSilence.WithContext(ctx).Delete(obj); err != nil {
		return err
	}
	return nil
}

func (recevicer *alertSilenceService) QuerySilence(ctx context.Context, req *types.IDRequest) (*model.AlertSilence, error) {
	return aSilence.WithContext(ctx).Where(aSilence.ID.Eq(int(req.ID))).First()
}

func (recevicer *alertSilenceService) ListSilence(ctx context.Context, req *types.AlertSilenceListRequest) (*types.AlertSilenceListResponse, error) {
	var (
		AlertSilences []*model.AlertSilence
		total         int64
		query         = aSilence.WithContext(ctx).UnderlyingDB()
	)

	if req.Cluster != "" {
		query = query.Where(aSilence.Cluster.Eq(req.Cluster))
	}
	if req.Status != nil {
		query = query.Where(aSilence.Status.Eq(*req.Status))
	}
	if !req.EndsAt.IsZero() {
		query = query.Where(aSilence.EndsAt.Lte(req.EndsAt))
	}
	if !req.StartsAt.IsZero() {
		query = query.Where(aSilence.StartsAt.Gte(req.StartsAt))
	}
	if len(req.Matchers) > 0 {
		mBytes, err := json.Marshal(req.Matchers)
		if err != nil {
			return nil, err
		}
		query = query.Where("JSON_CONTAINS(matchers, ?)", string(mBytes))
	}

	if req.CreatedBy != "" {
		query = query.Where(aSilence.CreatedBy.Eq(req.CreatedBy))
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	if req.Sort != "" && req.Direction != "" {
		order := fmt.Sprintf("%s %s", req.Sort, req.Direction)
		query.Order(order)
	}

	if req.PageSize == 0 || req.Page == 0 {
		return nil, fmt.Errorf("pageSize 和 page 不能为0")
	}

	if err := query.Limit(req.PageSize).Offset((req.Page - 1) * req.PageSize).Find(&AlertSilences).Error; err != nil {
		return nil, err
	}

	return types.NewAlertSilenceListResponse(AlertSilences, total, req.PageSize, req.Page), nil
}

type CleanExpiredSilencer interface {
	CleanExpiredSilencesTask()
}

type CleanExpiredSilence struct {
	cache store.CacheLocker
}

func NewCleanExpiredSilencer(cache store.CacheStorer) CleanExpiredSilencer {
	return &CleanExpiredSilence{
		cache: cache,
	}
}

// CleanExpiredSilencesTask 定时任务：清理过期的静默规则
func (recevicer *CleanExpiredSilence) CleanExpiredSilencesTask() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	lockExpiration := 5 * time.Minute
	ok, err := recevicer.cache.SetNX(ctx, store.LockType, constant.AlertCleanExpiredSilencesLockKey, time.Now().Unix(), lockExpiration)
	if err != nil {
		zap.L().Error("[定时任务] Redis 分布式锁异常", zap.Error(err))
		return
	}
	if !ok {
		zap.L().Debug("[定时任务] 清理过期静默规则任务正在其他节点运行，本次跳过")
		return
	}
	zap.L().Info("[定时任务] 成功获取锁，开始清理过期静默规则")

	now := time.Now()
	// --- 逻辑 A: 将已过期的规则状态从 1 改为 0 ---
	// 逻辑：如果结束时间早于现在，且状态还是“启用”，则更新为“禁用/过期”
	info, err := aSilence.WithContext(ctx).
		Where(aSilence.Status.Eq(model.SilenceEnabled)).
		Where(aSilence.EndsAt.Lt(now)).
		Update(aSilence.Status, model.SilenceExpired)

	if err != nil {
		zap.L().Error("[定时任务] 更新过期静默规则状态失败", zap.Error(err))
	} else if info.RowsAffected > 0 {
		zap.L().Info("[定时任务] 成功将过期静默规则置为失效", zap.Int64("count", info.RowsAffected))
	}

	// // --- 逻辑 B: (可选) 物理删除过期很久的记录 (例如 30 天前) ---
	// // 这样可以防止数据表无限增长
	// thirtyDaysAgo := now.AddDate(0, 0, -30)
	// // 使用 Unscoped 进行硬删除，或者不加 Unscoped 进行软删除
	// delInfo, err := aSilence.WithContext(ctx).
	// 	Unscoped().
	// 	Where(aSilence.EndsAt.Lt(thirtyDaysAgo)).
	// 	Delete()

	// if err != nil {
	// 	zap.L().Error("[定时任务] 硬删除久远静默记录失败", zap.Error(err))
	// } else if delInfo.RowsAffected > 0 {
	// 	zap.L().Info("[定时任务] 物理清理久远静默记录成功", zap.Int64("count", delInfo.RowsAffected))
	// }
}
