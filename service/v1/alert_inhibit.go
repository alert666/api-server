package v1

import (
	"context"
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

	lockDuration := 1 * time.Minute
	ctx, cancle := context.WithTimeout(context.TODO(), 58*time.Second)
	defer cancle()

	ok, err := a.cacheImpl.SetNX(ctx, store.LockType, constant.AlertCleanInhibitLockKey, time.Now().Unix(), lockDuration)
	if err != nil {
		zap.L().Error("[定时任务] CleanInhibitAlert Redis 锁异常", zap.Error(err))
		return
	}

	// 没抢到锁，说明其他副本正在执行，直接退出
	if !ok {
		zap.L().Debug("[定时任务] CleanInhibitAlert 任务正在其他节点运行，本次跳过")
		return
	}
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
		zap.L().Info("不存在 firing 的告警", zap.Any("TargetsWhere", w.TargetsWhere))
		return nil
	}

	var startsAt time.Time
	for i, v := range tStoreObjs {
		if i == 0 {
			startsAt = v.StartsAt
		}

		if v.StartsAt.After(startsAt) {
			startsAt = v.StartsAt
		}
	}

	sQuery = sQuery.Where("starts_at >= ?", startsAt)
	for _, v := range w.SourcesWhere {
		sQuery = sQuery.Where(v.ColumnExpr, v.Value)
	}
	if err := sQuery.Find(&sStoreObjs).Error; err != nil {
		return fmt.Errorf("查询 source alertHistory 失败, %w", err)
	}

	if len(sStoreObjs) == 0 {
		zap.L().Info("不存在 firing 的告警", zap.Any("sourceLable", w.SourcesWhere))
		return nil
	}

	tStoreObjsSet := make(map[string][]*model.AlertHistory, len(tStoreObjs))
	for _, v := range tStoreObjs {
		tStoreObjsSet[v.Cluster] = append(tStoreObjsSet[v.Cluster], v)
	}

	for _, v := range sStoreObjs {
		if tObjs, ok := tStoreObjsSet[v.Cluster]; ok {
			for i := range tObjs {
				if tObjs[i].Status == constant.AlertStatusFiring {
					tObjs[i].Status = constant.AlertStatusResolved
				}
			}
		}
	}

	var updateObjs []*model.AlertHistory
	for _, v := range tStoreObjsSet {
		updateObjs = append(updateObjs, v...)
	}

	if len(sStoreObjs) == 0 {
		return nil
	}

	if _, err := aHistory.WithContext(ctx).Updates(updateObjs); err != nil {
		return fmt.Errorf("更新被抑制告警失败, %w", err)
	}
	return nil
}
