package server

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/alert666/api-server/base/bind"
	"github.com/alert666/api-server/base/conf"
	"github.com/alert666/api-server/base/constant"
	"github.com/alert666/api-server/base/router"
	apitypes "github.com/alert666/api-server/base/types"
	v1 "github.com/alert666/api-server/service/v1"
	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

const (
	defaultShutdownTimeout = 30 * time.Second
)

type ServerInterface interface {
	Start() error
	Stop() error
}

type Server struct {
	shutdown time.Duration
	server   *http.Server
}

func NewServer(server *gin.Engine) *Server {
	return &Server{
		shutdown: defaultShutdownTimeout,
		server: &http.Server{
			Addr:    conf.GetServerBind(),
			Handler: server,
		},
	}
}

func (s *Server) Start() (err error) {
	zap.S().Infof("start server, addr: %s", s.server.Addr)
	if err = s.server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), s.shutdown)
	defer cancel()
	return s.server.Shutdown(ctx)
}

func NewHttpServer(r router.RouterInterface) (*gin.Engine, error) {
	if conf.GetLogLevel() == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}
	engine := gin.New()
	bind.NewValidator()

	// 注册路由
	r.RegisterRouter(engine)
	var apiData apitypes.ServerApiData
	apiData.ApiInfo = make(map[string][]apitypes.ApiInfo)
	for _, v := range engine.Routes() {
		if v.Path == "/swagger/*any" || v.Path == "/oauth2/login" || v.Path == "/oauth2/callback" || v.Path == "/oauth2/provider" {
			continue
		}
		api := strings.TrimPrefix(v.Path, "/")
		apiType := strings.Split(api, "/")[2]
		_, ok := apiData.ApiInfo[apiType]
		if !ok {
			apiData.ApiInfo[apiType] = make([]apitypes.ApiInfo, 0)
			apiData.ApiType = append(apiData.ApiType, apiType)
		}
		apiData.ApiInfo[apiType] = append(apiData.ApiInfo[apiType], apitypes.ApiInfo{
			Method:  v.Method,
			Path:    v.Path,
			Handler: v.Handler,
		})
	}
	constant.ApiData = apiData
	return engine, nil
}

type CronJob struct {
	c                      *cron.Cron
	stopChan               chan struct{}
	shutdown               time.Duration
	cleanDuplicateFiringer v1.CleanDuplicateFiringer
	cleanExpiredSilencer   v1.CleanExpiredSilencer
	cleanInhibitAlert      v1.AlertInhibiter
}

func NewCronJob(
	cleanDuplicateFiringer v1.CleanDuplicateFiringer,
	cleanExpiredSilencer v1.CleanExpiredSilencer,
	cleanInhibitAlert v1.AlertInhibiter,
) *CronJob {
	return &CronJob{
		shutdown:               defaultShutdownTimeout,
		cleanDuplicateFiringer: cleanDuplicateFiringer,
		cleanExpiredSilencer:   cleanExpiredSilencer,
		cleanInhibitAlert:      cleanInhibitAlert,
	}
}

type jobConfig struct {
	name string
	spec string
	fn   func()
}

func (receiver *CronJob) Start() error {
	receiver.stopChan = make(chan struct{})
	c := cron.New(cron.WithChain(
		cron.SkipIfStillRunning(cron.DefaultLogger),
	))

	jobs := []jobConfig{
		{
			name: "抑制告警清理",
			spec: "* * * * *",
			fn:   receiver.cleanInhibitAlert.CleanInhibitAlert,
		},
		{
			name: "重复指纹告警清理",
			spec: "*/10 * * * *",
			fn:   receiver.cleanDuplicateFiringer.CleanDuplicateFiringAlertsTask,
		},
		{
			name: "重复告警静默清理",
			spec: "* * * * *",
			fn:   receiver.cleanExpiredSilencer.CleanExpiredSilencesTask,
		},
	}

	for _, job := range jobs {
		// 显式捕获局部变量，确保闭包安全
		currJob := job
		_, err := c.AddFunc(currJob.spec, func() {
			zap.L().Info("[CronJob] 任务开始执行", zap.String("job", currJob.name))
			currJob.fn()
		})

		if err != nil {
			zap.L().Error("注册定时任务失败", zap.String("job", currJob.name), zap.Error(err))
			return err
		}
		zap.L().Info("注册定时任务成功", zap.String("job", currJob.name), zap.String("spec", currJob.spec))
	}

	c.Start()
	receiver.c = c
	<-receiver.stopChan
	return nil
}

func (receiver *CronJob) Stop() error {
	zap.L().Info("正在停止定时任务调度器...")

	// 1. 先安全停止 cron 调度器（这是 robfig/cron 提供的优雅停止）
	if receiver.c != nil {
		stopCtx := receiver.c.Stop()
		// 等待正在跑的任务执行完，或者你可以加个 timeout
		<-stopCtx.Done()
	}

	// 2. 增加安全保护，防止重复关闭 channel
	// 最好在 NewCronJob 的时候就初始化 stopChan，而不是在 Start 里初始化
	if receiver.stopChan != nil {
		// 使用 select 确保只关闭一次，或者简单的逻辑判断
		select {
		case <-receiver.stopChan:
			// 已经关闭过了，啥都不做
		default:
			close(receiver.stopChan)
		}
	}

	zap.L().Info("定时任务调度器已安全停止")
	return nil
}
