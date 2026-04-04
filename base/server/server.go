package server

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qinquanliuxiang666/alertmanager/base/conf"
	"github.com/qinquanliuxiang666/alertmanager/base/constant"
	"github.com/qinquanliuxiang666/alertmanager/base/router"
	apitypes "github.com/qinquanliuxiang666/alertmanager/base/types"
	"github.com/qinquanliuxiang666/alertmanager/controller"
	v1 "github.com/qinquanliuxiang666/alertmanager/service/v1"
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
	controller.NewValidator()

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

type CronJobNew struct {
	c                      *cron.Cron
	stopChan               chan struct{}
	shutdown               time.Duration
	CleanDuplicateFiringer v1.CleanDuplicateFiringer
}

func NewCleanDuplicateFiringer(cleanDuplicateFiringer v1.CleanDuplicateFiringer) *CronJobNew {
	return &CronJobNew{
		shutdown:               defaultShutdownTimeout,
		CleanDuplicateFiringer: cleanDuplicateFiringer,
	}
}
func (receiver *CronJobNew) Start() error {
	receiver.stopChan = make(chan struct{})
	c := cron.New(cron.WithChain(
		cron.SkipIfStillRunning(cron.DefaultLogger),
	))
	_, err := c.AddFunc("*/10 * * * *", func() {
		zap.L().Info("[CronJob] 开始执行重复告警清理任务...")
		receiver.CleanDuplicateFiringer.CleanDuplicateFiringAlertsTask()
	})
	if err != nil {
		zap.L().Fatal("注册定时任务失败", zap.Error(err))
		return err
	}
	c.Start()
	zap.L().Info("定时任务调度器已启动，每 10 分钟执行一次清理")
	receiver.c = c
	<-receiver.stopChan
	return nil
}

func (receiver *CronJobNew) Stop() error {
	zap.L().Info("正在停止定时任务调度器...")
	if receiver.c != nil {
		stopCtx := receiver.c.Stop()
		<-stopCtx.Done()
	}
	close(receiver.stopChan)
	zap.L().Info("定时任务调度器已安全停止")
	return nil
}
