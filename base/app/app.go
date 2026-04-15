package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/alert666/api-server/base/constant"
	"github.com/alert666/api-server/base/helper"
	"github.com/alert666/api-server/base/server"
	"github.com/alert666/api-server/base/types"
	"github.com/alert666/api-server/model"
	"github.com/alert666/api-server/pkg/feishu"
	v1 "github.com/alert666/api-server/service/v1"
	"github.com/alert666/api-server/store"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Options func(*Application)

func WithServer(server ...server.ServerInterface) Options {
	return func(app *Application) {
		app.servers = append(app.servers, server...)
	}
}

func WithInit(redis store.CacheStorer, feishu feishu.Feishuer) Options {
	return func(app *Application) {
		app.Initer = &Init{
			caceImpl:   redis,
			feishuImpl: feishu,
		}
	}
}

// Application 所有依赖集合
type Application struct {
	servers []server.ServerInterface
	wg      *sync.WaitGroup
	Initer  Initer
}

// Initer 初始化接口
type Initer interface {
	Init(ctx context.Context) error
}

// Init Initer 的实现
type Init struct {
	caceImpl   store.CacheStorer
	feishuImpl feishu.Feishuer
}

func (receiver *Init) Init(ctx context.Context) error {
	// 1. 从数据库获取全量数据（包含关联的模板）, 缓存到 cache
	alertChannels, err := store.AlertChannel.
		Preload(store.AlertChannel.AlertTemplate).
		Where(store.AlertChannel.Status.Eq(int(model.StatusEnabled))).
		Find()
	if err != nil {
		return fmt.Errorf("获取全量 alertChannel 失败: %w", err)
	}
	for _, v := range alertChannels {
		err := receiver.caceImpl.SetObject(ctx, store.AlertType, v.Name, v, store.NeverExpires)
		if err != nil {
			zap.L().Error("同步 AlertChannel 到 Redis 失败", zap.String("name", v.Name), zap.Error(err))
			continue
		}
		zap.L().Info("同步 AlertChannel 到 Redis 成功", zap.Any("channels", alertChannels))

		var alertConfig map[string]string
		if err := json.Unmarshal(v.Config, &alertConfig); err != nil {
			zap.L().Error("序列化 AlertChannel 配置失败", zap.String("name", v.Name), zap.Error(err))
			continue
		}

		switch v.Type {
		case model.ChannelTypeFeishuApp:
			appID := alertConfig["app_id"]
			appSecret := alertConfig["app_secret"]

			if appID == "" || appSecret == "" {
				zap.L().Warn("飞书应用配置不完整", zap.String("name", v.Name))
				continue
			}

			// 初始化飞书 SDK 客户端到内存
			receiver.feishuImpl.Init(v.Name, appID, appSecret)
			zap.L().Info("飞书客户端初始化成功", zap.String("channel", v.Name))

		case model.ChannelTypeFeishuBoot:
			// 如果有机器人逻辑可以在此扩展

		default:
			zap.L().Info("跳过非 SDK 类型的渠道初始化", zap.String("type", string(v.Type)))
		}
	}

	// 2. 缓存 tenants
	zap.L().Info("缓存全量 tenants")
	storeTenants, err := store.Tenant.WithContext(ctx).Find()
	if err != nil {
		return fmt.Errorf("获取全量 tenant 失败: %w", err)
	}

	res := make([]*types.TenantOption, 0, len(storeTenants))
	for _, storeObj := range storeTenants {
		res = append(res, &types.TenantOption{
			Label: storeObj.Name,
			Value: storeObj.Name,
		})
	}

	if err := receiver.caceImpl.SetObject(ctx, store.TenantType, constant.TenantOptionsCacheKey, res, store.NeverExpires); err != nil {
		zap.L().Error("缓存 tenants 失败", zap.Error(err))
	}

	// 3. 订阅 alertChannel 删除事件
	zap.L().Info("订阅 alertChannel 删除事件")
	receiver.caceImpl.Subscribe(ctx, constant.AlertChannelTopicDelete, func(msg string) {
		cctx, cannelFc := context.WithTimeout(context.Background(), time.Second*10)
		defer cannelFc()
		_, err := store.AlertChannel.WithContext(cctx).Where(store.AlertChannel.Name.Eq(msg)).First()
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				receiver.feishuImpl.CloseCli(msg)
				return
			}
			zap.L().Error(fmt.Sprintf("订阅 alertChannel 删除事件, 查询 name = %s 的alertChannel 失败", msg), zap.Error(err))
			return
		}
		zap.S().Errorf("订阅 alertChannel 删除事件, 数据库存在记录 name = %s 的 alertChannel 删除失败", msg)
	})

	// 4. 订阅 alertChannel 更新事件
	zap.L().Info("订阅 alertChannel 更新事件")
	receiver.caceImpl.Subscribe(ctx, constant.AlertChannelTopicUpdate, func(msg string) {
		cctx, cannelFc := context.WithTimeout(context.Background(), time.Second*10)
		defer cannelFc()
		channel, err := store.AlertChannel.WithContext(cctx).Where(store.AlertChannel.Name.Eq(msg)).First()
		if err != nil {
			zap.L().Error("订阅 alertChannel 更新事件, 查询 alertChannel 失败", zap.Error(err))
			return
		}

		if *channel.Status == model.StatusDisabled {
			receiver.feishuImpl.CloseCli(channel.Name)
			return
		}

		switch channel.Type {
		case model.ChannelTypeFeishuApp:
			appid, appSecret, err := helper.VerificationAlertFeishuConfig(channel)
			if err != nil {
				zap.S().Error(err)
				return
			}
			receiver.feishuImpl.UpdateCli(msg, appid, appSecret)
			return
		}
	})

	return nil
}

func newApp(options ...Options) *Application {
	app := &Application{
		wg: &sync.WaitGroup{},
	}
	for _, option := range options {
		option(app)
	}
	return app
}

func NewApplication(
	e *gin.Engine,
	redis store.CacheStorer,
	feishu feishu.Feishuer,
	cleanDuplicateFiringer v1.CleanDuplicateFiringer,
	cleanExpiredSilencer v1.CleanExpiredSilencer,
) *Application {
	return newApp(
		WithServer(
			server.NewServer(e),
			server.NewCronJob(cleanDuplicateFiringer, cleanExpiredSilencer),
		),
		WithInit(redis, feishu),
	)
}

func (app *Application) Run(ctx context.Context) error {
	if len(app.servers) == 0 {
		return nil
	}
	errCh := make(chan error, 1)
	for _, s := range app.servers {
		go func(s server.ServerInterface) {
			errCh <- s.Start()
		}(s)
	}

	select {
	case err := <-errCh:
		app.Stop()
		return err
	case <-ctx.Done():
		app.Stop()
		return nil
	}
}

func (app *Application) Stop() {
	if len(app.servers) == 0 {
		return
	}
	for _, s := range app.servers {
		app.wg.Add(1)
		go func(s server.ServerInterface) {
			defer app.wg.Done()
			if err := s.Stop(); err != nil {
				zap.S().Errorf("stop server error %v", err)
			}
		}(s)
	}
	app.wg.Wait()
}
