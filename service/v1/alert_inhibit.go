package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/alert666/api-server/base/constant"
	"github.com/alert666/api-server/model"
	alertinhibit "github.com/alert666/api-server/pkg/alertinhibit"
	"github.com/alert666/api-server/store"
	"go.uber.org/zap"
)

type AlertInhibiter interface {
	CleanInhibitAlert()
}

type alertInhibit struct {
	matchersImpl []*alertinhibit.InhibitMatcher
	cacheImpl    store.CacheStorer
}

func NewalertInhibit(inhibitMatchers []*alertinhibit.InhibitMatcher, cache store.CacheStorer) AlertInhibiter {
	return &alertInhibit{
		matchersImpl: inhibitMatchers,
		cacheImpl:    cache,
	}
}

// CleanInhibitAlert 定时任务 清理被抑制的且没有恢复的告警
func (a *alertInhibit) CleanInhibitAlert() {
	start := time.Now()
	defer func() {
		elapsed := time.Since(start).Milliseconds()
		if r := recover(); r != nil {
			stack := debug.Stack()
			zap.L().Error("CleanInhibitAlert panic recovered",
				zap.Any("panic", r),
				zap.String("stack", string(stack)), // 这行会告诉你具体是代码哪一行崩了
			)
			return
		}

		zap.L().Info("CleanInhibitAlert 执行结束",
			zap.Int64("duration_ms", elapsed),
		)
	}()

	ctx, cancle := context.WithTimeout(context.TODO(), 58*time.Minute)
	defer cancle()

	ok, err := a.cacheImpl.SetNX(ctx, store.LockType, constant.AlertCleanInhibitLockKey, time.Now().Unix(), 58*time.Second)
	if err != nil {
		zap.L().Error("[定时任务] CleanInhibitAlert Redis 锁异常", zap.Error(err))
		return
	}
	defer a.cacheImpl.DelKey(ctx, store.LockType, constant.AlertCleanInhibitLockKey)

	// 没抢到锁，说明其他副本正在执行，直接退出
	if !ok {
		zap.L().Debug("[定时任务] CleanInhibitAlert 任务正在其他节点运行，本次跳过")
		return
	}

	zap.L().Info("[定时任务] 成功获取锁，开始清理被抑制告警")
	// 获取源 label 相关的告警
	var wg sync.WaitGroup
	for _, m := range a.matchersImpl {
		wg.Add(1)
		go func(matcher *alertinhibit.InhibitMatcher) {
			defer wg.Done()
			inhibitWhere, err := matcher.Match()
			if err != nil {
				zap.L().Error("转换抑制规则为查询语句失败", zap.Error(err))
				return
			}
			if err = a.getAlert(ctx, inhibitWhere); err != nil {
				zap.L().Error("处理被抑制告警失败", zap.Error(err))
			}
		}(m)
	}
	wg.Wait()
}

func (a *alertInhibit) getAlert(ctx context.Context, w *alertinhibit.InhibitWhere) error {
	var (
		sQuery     = aHistory.UnderlyingDB().WithContext(ctx).Where("status = ?", constant.AlertStatusResolved)
		sStoreObjs []*model.AlertHistory
		tQuery     = aHistory.UnderlyingDB().WithContext(ctx).Where("status = ?", constant.AlertStatusFiring)
		tStoreObjs []*model.AlertHistory
	)

	for _, v := range w.TargetsWhere {
		tQuery = tQuery.Where(v.ColumnExpr, v.Value)
	}
	if err := tQuery.Find(&tStoreObjs).Error; err != nil {
		return fmt.Errorf("查询 target alertHistory 失败, %w", err)
	}

	if len(tStoreObjs) == 0 {
		zap.L().Debug("不存在 target_matchers 状态为 resolved 的告警, 定时任务退出", zap.Any("TargetsWhere", w.TargetsWhere))
		return nil
	}

	var minStartsAt time.Time
	for i, v := range tStoreObjs {
		if i == 0 || v.StartsAt.Before(minStartsAt) {
			minStartsAt = v.StartsAt
		}
	}

	sQuery = sQuery.Where("starts_at >= ?", minStartsAt)
	for _, v := range w.SourcesWhere {
		sQuery = sQuery.Where(v.ColumnExpr, v.Value)
	}
	if err := sQuery.Find(&sStoreObjs).Error; err != nil {
		return fmt.Errorf("查询 source alertHistory 失败, %w", err)
	}

	if len(sStoreObjs) == 0 {
		zap.L().Debug("不存在 source_matchers 状态为 firing 的告警, 定时任务退出", zap.Any("sourceLable", w.SourcesWhere))
		return nil
	}

	tStoreObjsSet := make(map[string][]*model.AlertHistory, len(tStoreObjs))
	for _, v := range tStoreObjs {
		tStoreObjsSet[v.Cluster] = append(tStoreObjsSet[v.Cluster], v)
	}

	var inhibitedIDs []int
	for _, sObj := range sStoreObjs {
		var sLabelSet map[string]string
		if err := json.Unmarshal(sObj.Labels, &sLabelSet); err != nil {
			return fmt.Errorf("序列化 sObj label 失败, %w", err)
		}

		targets, ok := tStoreObjsSet[sObj.Cluster]
		if !ok {
			continue
		}

		for _, tObj := range targets {
			// 如果该 Target 已经标记为待处理，跳过
			if tObj.Status == constant.AlertStatusResolved {
				continue
			}
			var dLabelSet map[string]string
			if err := json.Unmarshal(tObj.Labels, &dLabelSet); err != nil {
				return fmt.Errorf("序列化 tObj label 失败, %w", err)
			}

			// Equal 标签匹配逻辑 ---
			isEqual := true
			for _, labelKey := range w.Equal {
				sValue, ok := sLabelSet[labelKey]
				if !ok {
					isEqual = false
					break
				}

				dValue, ok := dLabelSet[labelKey]
				if !ok {
					isEqual = false
					break
				}

				if sValue != dValue {
					isEqual = false
					break
				}
			}

			if isEqual {
				tObj.Status = constant.AlertStatusResolved
				inhibitedIDs = append(inhibitedIDs, tObj.ID)
			}
		}
	}

	if len(inhibitedIDs) == 0 {
		return nil
	}

	now := time.Now()
	err := aHistory.UnderlyingDB().WithContext(ctx).
		Model(&model.AlertHistory{}).
		Where("id IN ?", inhibitedIDs).
		Updates(map[string]interface{}{
			"status":  constant.AlertStatusResolved,
			"ends_at": now,
		}).Error
	if err != nil {
		return fmt.Errorf("批量更新抑制告警状态失败, IDs: %v, %w", inhibitedIDs, err)
	}

	zap.L().Info("成功通过抑制规则恢复告警记录",
		zap.Int("count", len(inhibitedIDs)),
		zap.Ints("ids", inhibitedIDs),
	)

	return nil
}
