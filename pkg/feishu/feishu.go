package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"text/template"
	"time"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	"github.com/qinquanliuxiang666/alertmanager/base/conf"
	"github.com/qinquanliuxiang666/alertmanager/base/constant"
	"github.com/qinquanliuxiang666/alertmanager/base/helper"
	"github.com/qinquanliuxiang666/alertmanager/base/log"
	"github.com/qinquanliuxiang666/alertmanager/base/types"
	"github.com/qinquanliuxiang666/alertmanager/model"
)

var feishuStruct = &FeiShu{
	clients: make(map[string]*Client),
}

func printFeishuCli() string {
	clientNames := make([]string, 0, len(feishuStruct.clients))
	for name := range feishuStruct.clients {
		clientNames = append(clientNames, name)
	}
	return strings.Join(clientNames, ",")
}

type Feishuer interface {
	Init(alertChannelName, appid, appSecret string)
	GetCli(alertChannelName string) (*lark.Client, error)
	UpdateCli(alertChannelName, appid, appSecret string)
	CloseCli(alertChannelName string)
	Notifyer
}

type Notifyer interface {
	Notify(ctx context.Context, notifyReq *types.NotifyReq) (result *types.NotifySendResult, err error)
}

type FeiShu struct {
	lock    sync.Mutex
	clients map[string]*Client
}

type Client struct {
	cli      *lark.Client
	wsCli    *larkws.Client
	cancelFn context.CancelFunc
}

func NewFeiShu() Feishuer {
	return feishuStruct
}

func (receiver *FeiShu) Init(alertChannelName, appid, appSecret string) {
	receiver.lock.Lock()
	defer receiver.lock.Unlock()
	if _, ok := feishuStruct.clients[alertChannelName]; ok {
		return
	}

	cli, wsCli, cancelFn := newFeishuClient(alertChannelName, appid, appSecret)
	receiver.clients[alertChannelName] = &Client{
		cli:      cli,
		wsCli:    wsCli,
		cancelFn: cancelFn,
	}
	clientNames := printFeishuCli()
	zap.S().Infof("初始化新的飞书客户端 %s, 当前已缓存的客户端 %s", alertChannelName, clientNames)
}

func (receiver *FeiShu) GetCli(alertChannelName string) (*lark.Client, error) {
	receiver.lock.Lock()
	defer receiver.lock.Unlock()

	c, ok := receiver.clients[alertChannelName]
	if !ok {
		return nil, fmt.Errorf("client %s not initialized", alertChannelName)
	}
	return c.cli, nil
}

func newFeishuClient(alertChannelName, appid, appSecret string) (*lark.Client, *larkws.Client, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	eventHandler := dispatcher.NewEventDispatcher("", "").
		OnP2CardActionTrigger(func(ctx context.Context, event *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error) {
			feiShuCardTrigger := new(helper.FeiShuCardTrigger)
			if err := json.Unmarshal(event.Body, feiShuCardTrigger); err != nil {
				return nil, err
			}
			fmt.Println("☀️------------------------------------☀️")
			v, ok := feiShuCardTrigger.Event.Action.Value[feiShuCardTrigger.Event.Action.Option]
			if ok {
				fmt.Printf("v: %s", v)
			}
			fmt.Println("🌙------------------------------------🌙")

			return nil, nil
			// return &callback.CardActionTriggerResponse{
			// 	Toast: &callback.Toast{
			// 		Type:    "info",
			// 		Content: "静默成功",
			// 	},
			// 	Card: &callback.Card{
			// 		Type: "template",
			// 		Data: map[string]any{
			// 			"template_id": "AAqK947a7l70i",
			// 			"template_variable": map[string]any{
			// 				"disableSelect": true,
			// 			},
			// 		},
			// 	},
			// }, nil
		}).
		// 监听「拉取链接预览数据 url.preview.get」
		OnP2CardURLPreviewGet(func(ctx context.Context, event *callback.URLPreviewGetEvent) (*callback.URLPreviewGetResponse, error) {
			// fmt.Printf("[ OnP2URLPreviewAction access ], data: %s\n", larkcore.Prettify(event))
			evebtByte, err := json.Marshal(event)
			if err != nil {
				panic(err)
			}

			fmt.Println("☀️------------------------------------☀️")
			fmt.Println(string(evebtByte))
			fmt.Println("🌙------------------------------------🌙")
			return nil, nil
		})
	// 创建Client

	var larkLogLevel larkcore.LogLevel
	if conf.GetLogLevel() == "debug" {
		larkLogLevel = larkcore.LogLevelDebug
	} else {
		larkLogLevel = larkcore.LogLevelInfo
	}

	wsCli := larkws.NewClient(appid, appSecret,
		larkws.WithEventHandler(eventHandler),
		larkws.WithLogLevel(larkLogLevel),
	)
	zap.S().Infof("创建新的飞书客户端 %s 长连接", alertChannelName)

	go func() {
		err := wsCli.Start(ctx)
		if err != nil {
			if err == context.Canceled {
				zap.L().Info("lark WS Connection closed by cancelFn", zap.String("app_id", appid))
				return
			}
			zap.L().Error("lark WS Start Error", zap.Error(err))
		}
	}()

	cli := lark.NewClient(appid, appSecret,
		lark.WithLogLevel(larkcore.LogLevelDebug),
		lark.WithReqTimeout(10*time.Second),
	)

	clientNames := printFeishuCli()
	zap.S().Infof("创建新的飞书客户端 %s, 当前已缓存的客户端 %s", alertChannelName, clientNames)
	return cli, wsCli, cancel
}

// UpdateCli 如果 appid 和 appSecret 修改需要重新初始化客户端
func (receiver *FeiShu) UpdateCli(alertChannelName, appid, appSecret string) {
	receiver.lock.Lock()
	defer receiver.lock.Unlock()

	// 1. 如果旧客户端存在，先执行销毁逻辑
	if oldClient, ok := receiver.clients[alertChannelName]; ok {
		zap.S().Info("正在更新通道 [%s] 的客户端，关闭旧连接...", alertChannelName)
		if oldClient.cancelFn != nil {
			// 触发 context 取消，停止旧的 wsCli.Start
			oldClient.cancelFn()
			zap.S().Infof("停止旧的飞书客户端 %s 连接, ", alertChannelName)
		}
	}

	time.Sleep(5 * time.Second)
	// 2. 创建新的客户端
	cli, wsCli, cancel := newFeishuClient(alertChannelName, appid, appSecret)

	// 3. 覆盖存储
	receiver.clients[alertChannelName] = &Client{
		cli:      cli,
		wsCli:    wsCli,
		cancelFn: cancel,
	}
	clientNames := printFeishuCli()
	zap.S().Infof("已更新飞书客户端 %s, 当前已缓存的客户端 %s", alertChannelName, clientNames)
}

// UpdateCli 如果 appid 和 appSecret 修改需要重新初始化客户端
func (receiver *FeiShu) CloseCli(alertChannelName string) {
	receiver.lock.Lock()
	defer receiver.lock.Unlock()

	// 1. 如果旧客户端存在，先执行销毁逻辑
	if oldClient, ok := receiver.clients[alertChannelName]; ok {
		zap.S().Infof("正在更新通道 [%s] 的客户端，关闭旧连接...", alertChannelName)
		if oldClient.cancelFn != nil {
			oldClient.cancelFn()
			time.Sleep(5 * time.Second)
		}
	}
	delete(receiver.clients, alertChannelName)
	clientNames := printFeishuCli()
	zap.S().Infof("从本地缓存中删除 [%s] 的客户端成功, 当前已缓存的客户端 %s", alertChannelName, clientNames)
}

func (receiver *FeiShu) renderAndSend(ctx context.Context, larkCli *lark.Client, conf *model.FeishuAppConfig, data interface{}, tpl string, color string) error {
	log.WithRequestID(ctx).Debug("飞书发送告警", zap.Any("data", data))
	// 1. 渲染模板
	content, err := RenderingAlertContent().Build(ctx, data, tpl)
	if err != nil {
		return err
	}

	// 2. 设置标题颜色 (如果模板里没写死的话)
	if content.Data.TemplateVariable == nil {
		content.Data.TemplateVariable = make(map[string]any)
	}
	content.Data.TemplateVariable["titleColor"] = color

	// 3. 序列化
	byData, err := json.Marshal(content)
	if err != nil {
		return fmt.Errorf("marshal失败: %w", err)
	}

	// 4. 发送
	return SendCard(larkCli).Build(ctx, conf.ReceiveIdType, conf.ReceiveId, string(byData))
}

type FeishuCard struct {
	cli *lark.Client
}

func SendCard(cli *lark.Client) *FeishuCard {
	return &FeishuCard{
		cli: cli,
	}
}

func (receiver *FeishuCard) Build(ctx context.Context, receiveIdType, receiveId, content string) error {
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(receiveIdType).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(receiveId).
			MsgType(`interactive`).
			Content(content).
			Build()).
		Build()

	// 发起请求
	resp, err := receiver.cli.Im.V1.Message.Create(ctx, req)

	// 处理错误
	if err != nil {
		return err
	}

	// 服务端错误处理
	if !resp.Success() {
		log.WithRequestID(ctx).Error("发起请求发送飞书卡片时服务发生错误", zap.String("logId", resp.RequestId()), zap.Error(err), zap.String("错误响应", larkcore.Prettify(resp.CodeError)))
		return err
	}

	// // 业务处理
	// fmt.Println(larkcore.Prettify(resp))
	return nil
}

// FeiShuContent 飞书卡片模版request
type FeiShuContent struct {
	Type string                `json:"type"`
	Data FeishuCardDataContent `json:"data"`
}

type FeishuCardDataContent struct {
	TemplateId          string         `json:"template_id"  yaml:"template_id"`
	TemplateNersionName string         `json:"template_version_name" yaml:"template_version_name"`
	TemplateVariable    map[string]any `json:"template_variable" yaml:"template_variable"`
}

func RenderingAlertContent() *FeishuCardDataContent {
	return &FeishuCardDataContent{}
}

type FeishuCardLink struct {
	PcUrl      string `yaml:"pc_url"`
	AndroidUrl string `yaml:"android_url"`
	IosUrl     string `yaml:"ios_url"`
	Url        string `yaml:"url"`
}

var funcMap = template.FuncMap{
	"timeFormat": func(t time.Time) string {
		var cstZone = time.FixedZone("CST", 8*3600)
		return t.In(cstZone).Format("2006-01-02 15:04:05")
	},
	"add": func(a, b int) int {
		return a + b
	},
	"getEndTime": func(endTime *time.Time, msg string) string {
		if endTime == nil || endTime.IsZero() {
			return msg
		}
		var cstZone = time.FixedZone("CST", 8*3600)
		return endTime.In(cstZone).Format("2006-01-02 15:04:05")
	},
	"newViewLink": func(link string) string {
		m := map[string]string{
			"pc_url":      link,
			"android_url": "",
			"ios_url":     "",
			"url":         link,
		}
		b, err := json.Marshal(m)
		if err != nil {
			return "{}"
		}
		return string(b)
	},
}

func (receiver *FeishuCardDataContent) Build(ctx context.Context, alert any, alertTpl string) (*FeiShuContent, error) {
	tmpl, err := template.New("alert").Funcs(funcMap).Parse(alertTpl)
	if err != nil {
		return nil, fmt.Errorf("构建告警模版失败, %s", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, alert); err != nil {
		return nil, fmt.Errorf("渲染告警模版失败, %s", err)
	}

	log.WithRequestID(ctx).Debug("告警模板", zap.String("data", buf.String()))

	if err := yaml.Unmarshal([]byte(buf.Bytes()), &receiver); err != nil {
		return nil, fmt.Errorf("序列化 FeishuCardDataContent 失败, %s", err)
	}

	return &FeiShuContent{
		Type: "template",
		Data: *receiver,
	}, nil
}

func (receiver *FeiShu) Notify(ctx context.Context, notifyReq *types.NotifyReq) (result *types.NotifySendResult, err error) {
	var (
		feishuAppConf *model.FeishuAppConfig
		alertChannel  = notifyReq.AlertChannel
	)

	feishuAppConf, err = alertChannel.GetFeishuAppConfig()
	if err != nil {
		return nil, fmt.Errorf("获取飞书配置失败: %w", err)
	}

	var firingErr, resolvedErr error
	larkCli, err := receiver.GetCli(alertChannel.Name)
	if err != nil {
		return nil, err
	}

	alertArry := notifyReq.AlertArry
	// 聚合发送告警
	if *notifyReq.AlertChannel.AggregationStatus == model.AggregationEnabled {
		log.WithRequestID(ctx).Debug("聚合发送告警")
		if len(alertArry.FiringAlertArry) > 0 {
			newReq := notifyReq.AlertReceiveReq.DeepCopy()
			newReq.Alerts = alertArry.FiringAlertArry
			firingErr = receiver.renderAndSend(ctx, larkCli, feishuAppConf, newReq, alertChannel.AlertTemplate.AggregationTemplate, "red")
			if firingErr != nil {
				log.WithRequestID(ctx).Error("聚合发送告警卡片失败", zap.Error(firingErr))
			}
		}

		if len(alertArry.ResolvedAlertArry) > 0 {
			newReq := notifyReq.AlertReceiveReq.DeepCopy()
			newReq.Alerts = alertArry.ResolvedAlertArry
			if resolvedErr = receiver.renderAndSend(ctx, larkCli, feishuAppConf, newReq, alertChannel.AlertTemplate.AggregationTemplate, "green"); resolvedErr != nil {
				log.WithRequestID(ctx).Error("聚合发送恢复卡片失败", zap.Error(resolvedErr))
			}
		}

		return &types.NotifySendResult{
			AggregationSendResult: &types.AggregationSendResult{
				FiringErr:   firingErr,
				ResolvedErr: resolvedErr,
			},
			SingleSendResult: nil,
		}, nil
	}

	if *notifyReq.AlertChannel.AggregationStatus == model.AggregationDisabled {
		log.WithRequestID(ctx).Debug("非聚合发送告警")
		// 非聚合发送
		normalSendResult, err := receiver.
			singleSend(ctx, larkCli, feishuAppConf, alertChannel, alertArry)
		if err != nil {
			return nil, err
		}
		return &types.NotifySendResult{
			AggregationSendResult: nil,
			SingleSendResult:      normalSendResult,
		}, nil
	}

	return nil, fmt.Errorf("不支持的发送模式, 只支持聚合发送和非聚合发送")
}

func (receiver *FeiShu) singleSend(ctx context.Context, larkCli *lark.Client, conf *model.FeishuAppConfig, alertChannel *model.AlertChannel, alertArry *types.AlertArry) ([]*types.SingleSendResult, error) {
	var errs []error
	var results []*types.SingleSendResult

	process := func(v *types.Alert) {
		color := "red"
		if v.Status == constant.AlertStatusResolved {
			color = "green"
		}
		err := receiver.renderAndSend(ctx, larkCli, conf, v, alertChannel.AlertTemplate.Template, color)

		results = append(results, &types.SingleSendResult{
			Alert:   v,
			SendErr: err,
		})

		if err != nil {
			log.WithRequestID(ctx).Error("发送单条飞书卡片失败", zap.Error(err))
			if len(errs) < 4 { // 限制错误收集数量
				errs = append(errs, err)
			}
		}
	}

	for _, v := range alertArry.FiringAlertArry {
		process(v)
	}
	for _, v := range alertArry.ResolvedAlertArry {
		process(v)
	}

	return results, errors.Join(errs...)
}
