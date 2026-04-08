package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/alert666/api-server/base/constant"
	"github.com/alert666/api-server/base/helper"
	"github.com/alert666/api-server/base/server"
	"github.com/alert666/api-server/model"
	"github.com/alert666/api-server/pkg/feishu"
	v1 "github.com/alert666/api-server/service/v1"
	"github.com/alert666/api-server/store"
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
			cace:   redis,
			feishu: feishu,
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
	cace   store.CacheStorer
	feishu feishu.Feishuer
}

func (receiver *Init) Init(ctx context.Context) error {
	// 1. 从数据库获取全量数据（包含关联的模板）
	alertChannels, err := store.AlertChannel.
		Preload(store.AlertChannel.AlertTemplate).
		Where(store.AlertChannel.Status.Eq(int(model.StatusEnabled))).
		Find()
	if err != nil {
		return fmt.Errorf("获取全量 alertChannel 失败: %w", err)
	}

	// 2. 遍历处理
	for _, v := range alertChannels {
		err := receiver.cace.SetObject(ctx, store.AlertType, v.Name, v, store.NeverExpires)
		if err != nil {
			zap.L().Error("同步 AlertChannel 到 Redis 失败", zap.String("name", v.Name), zap.Error(err))
			continue
		}

		// B. 解析配置
		var alertConfig map[string]string
		if err := json.Unmarshal(v.Config, &alertConfig); err != nil {
			zap.L().Error("序列化 AlertChannel 配置失败", zap.String("name", v.Name), zap.Error(err))
			continue
		}

		// C. 初始化具体的告警客户端 (如飞书)
		switch v.Type {
		case model.ChannelTypeFeishuApp:
			appID := alertConfig["app_id"]
			appSecret := alertConfig["app_secret"]

			if appID == "" || appSecret == "" {
				zap.L().Warn("飞书应用配置不完整", zap.String("name", v.Name))
				continue
			}

			// 初始化飞书 SDK 客户端到内存
			receiver.feishu.Init(v.Name, appID, appSecret)
			zap.L().Info("飞书客户端初始化成功", zap.String("channel", v.Name))

		case model.ChannelTypeFeishuBoot:
			// 如果有机器人逻辑可以在此扩展

		default:
			zap.L().Debug("跳过非 SDK 类型的渠道初始化", zap.String("type", string(v.Type)))
		}
	}

	zap.L().Info("告警渠道同步至 Redis 完成", zap.Int("count", len(alertChannels)))

	receiver.cace.Subscribe(ctx, constant.AlertChannelTopicDelete, func(msg string) {
		cctx, cannelFc := context.WithTimeout(context.Background(), time.Second*10)
		defer cannelFc()
		_, err := store.AlertChannel.WithContext(cctx).Where(store.AlertChannel.Name.Eq(msg)).First()
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				receiver.feishu.CloseCli(msg)
				return
			}
			zap.L().Error(fmt.Sprintf("订阅 alertChannel 删除事件, 查询 name = %s 的alertChannel 失败", msg), zap.Error(err))
			return
		}
		zap.S().Errorf("订阅 alertChannel 删除事件, 数据库存在记录 name = %s 的 alertChannel 删除失败", msg)
	})

	receiver.cace.Subscribe(ctx, constant.AlertChannelTopicUpdate, func(msg string) {
		cctx, cannelFc := context.WithTimeout(context.Background(), time.Second*10)
		defer cannelFc()
		channel, err := store.AlertChannel.WithContext(cctx).Where(store.AlertChannel.Name.Eq(msg)).First()
		if err != nil {
			zap.L().Error("订阅 alertChannel 更新事件, 查询 alertChannel 失败", zap.Error(err))
			return
		}

		if *channel.Status == model.StatusDisabled {
			receiver.feishu.CloseCli(channel.Name)
			return
		}

		switch channel.Type {
		case model.ChannelTypeFeishuApp:
			appid, appSecret, err := helper.VerificationAlertFeishuConfig(channel)
			if err != nil {
				zap.S().Error(err)
				return
			}
			receiver.feishu.UpdateCli(msg, appid, appSecret)
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
