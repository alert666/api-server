package v1

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/alert666/api-server/base/constant"
	"github.com/alert666/api-server/model"
	"github.com/alert666/api-server/store"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type CleanStaleCacher interface {
	CleanStaleCacheTask()
}

type cleanStaleCache struct {
	cacheImpl store.CacheStorer
}

func NewCleanStaleCacher(cache store.CacheStorer) CleanStaleCacher {
	return &cleanStaleCache{
		cacheImpl: cache,
	}
}

func (r *cleanStaleCache) CleanStaleCacheTask() {
	start := time.Now()
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			zap.L().Error("CleanStaleCacheTask panic recovered",
				zap.Any("panic", r),
				zap.String("stack", string(stack)),
			)
			return
		}
		elapsed := time.Since(start).Milliseconds()
		zap.L().Debug("CleanStaleCacheTask 执行结束",
			zap.Int64("duration_ms", elapsed),
		)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 58*time.Second)
	defer cancel()

	ok, err := r.cacheImpl.SetNX(ctx, store.LockType, constant.AlertCleanStaleCacheLockKey, time.Now().Unix(), 58*time.Second)
	if err != nil {
		zap.L().Error("[定时任务] CleanStaleCacheTask Redis 分布式锁异常", zap.Error(err))
		return
	}
	defer r.cacheImpl.DelKey(ctx, store.LockType, constant.AlertCleanStaleCacheLockKey)

	if !ok {
		zap.L().Debug("[定时任务] CleanStaleCacheTask 任务正在其他节点运行，本次跳过")
		return
	}
	zap.L().Debug("[定时任务] CleanStaleCacheTask 成功获取锁，开始清理孤儿缓存")

	r.cleanOrphanedChannels(ctx)
	r.cleanOrphanedTemplates(ctx)
}

func (r *cleanStaleCache) cleanOrphanedChannels(ctx context.Context) {
	keys, err := r.cacheImpl.ScanKeys(ctx, store.AlertChannelType)
	if err != nil {
		zap.L().Error("[定时任务] 扫描 AlertChannel 缓存失败", zap.Error(err))
		return
	}

	for _, key := range keys {
		id, err := strconv.Atoi(key)
		if err != nil {
			continue
		}
		_, err = aChannelStore.WithContext(ctx).Where(aChannelStore.ID.Eq(id)).First()
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				var cached model.AlertChannel
				_, _ = r.cacheImpl.GetObject(ctx, store.AlertChannelType, id, &cached)

				if err := r.cacheImpl.DelKey(ctx, store.AlertChannelType, id); err != nil {
					zap.L().Error("[定时任务] 删除 AlertChannel 缓存失败", zap.Int("id", id), zap.Error(err))
				}
				zap.L().Info("[定时任务] 清理已删除的 AlertChannel 缓存", zap.Int("id", id))

				if cached.Type == model.ChannelTypeFeishuApp {
					config, err := cached.GetFeishuAppConfig()
					if err == nil {
						publish := fmt.Sprintf("%s:%s:%s", cached.Name, config.AppID, config.AppSecret)
						_ = r.cacheImpl.Publish(ctx, constant.AlertChannelTopicDelete, publish)
					}
				}
			}
		}
	}
}

func (r *cleanStaleCache) cleanOrphanedTemplates(ctx context.Context) {
	keys, err := r.cacheImpl.ScanKeys(ctx, store.AlertTemplateType)
	if err != nil {
		zap.L().Error("[定时任务] 扫描 AlertTemplate 缓存失败", zap.Error(err))
		return
	}

	for _, key := range keys {
		_, err = aTemlpateStore.WithContext(ctx).Where(aTemlpateStore.Name.Eq(key)).First()
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				if err := r.cacheImpl.DelKey(ctx, store.AlertTemplateType, key); err != nil {
					zap.L().Error("[定时任务] 删除 AlertTemplate 缓存失败", zap.String("name", key), zap.Error(err))
				}
				zap.L().Info("[定时任务] 清理已删除的 AlertTemplate 缓存", zap.String("name", key))
			}
		}
	}
}
