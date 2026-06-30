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

	"github.com/alert666/api-server/base/conf"
	"github.com/alert666/api-server/base/constant"
	"github.com/alert666/api-server/base/helper"
	"github.com/alert666/api-server/base/log"
	"github.com/alert666/api-server/base/types"
	"github.com/alert666/api-server/model"
	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
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
	GetCli(alertChannelName, appid, appSecret string) (*lark.Client, error)
	UpdateCli(alertChannelName, appid, appSecret string)
	CloseCli(alertChannelName, appid, appSecret string)
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

	zap.L().Info("缓存飞书 app", zap.String("alertChannelName", alertChannelName), zap.String("appid", appid), zap.String("appSecret", appSecret))

	// 判断客户端是否存在
	hashStr := helper.HashFeishuAppConfig(appid, appSecret)
	if _, ok := feishuStruct.clients[hashStr]; ok {
		return
	}

	// 创建新的客户端并缓存
	cli, wsCli, cancelFn := newFeishuClient(alertChannelName, appid, appSecret)
	receiver.clients[hashStr] = &Client{
		cli:      cli,
		wsCli:    wsCli,
		cancelFn: cancelFn,
	}

	clientNames := printFeishuCli()
	zap.L().Info("初始化新的飞书客户端", zap.String("alertChannelName", alertChannelName), zap.String("clientNames", clientNames))
}

func (receiver *FeiShu) GetCli(alertChannelName, appid, appSecret string) (*lark.Client, error) {
	hashStr := helper.HashFeishuAppConfig(appid, appSecret)
	receiver.lock.Lock()
	defer receiver.lock.Unlock()
	zap.L().Debug("获取告警通道", zap.String("name", alertChannelName))

	c, ok := receiver.clients[hashStr]
	if !ok {
		return nil, fmt.Errorf("client %s not initialized", hashStr)
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

	zapAdapter := newZapLoggerAdapter(zap.L())
	wsCli := larkws.NewClient(appid, appSecret,
		larkws.WithEventHandler(eventHandler),
		larkws.WithLogLevel(larkLogLevel),
		larkws.WithLogger(zapAdapter),
	)
	zap.L().Info("创建新的飞书客户端长连接", zap.String("alertChannelName", alertChannelName))

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
		lark.WithLogLevel(larkLogLevel),
		lark.WithLogger(zapAdapter),
		lark.WithReqTimeout(10*time.Second),
	)

	clientNames := printFeishuCli()
	zap.L().Info("创建新的飞书客户端", zap.String("alertChannelName", alertChannelName), zap.String("clientNames", clientNames))
	return cli, wsCli, cancel
}

// UpdateCli 如果 appid 和 appSecret 修改需要重新初始化客户端
func (receiver *FeiShu) UpdateCli(alertChannelName, appid, appSecret string) {
	// 1. 先计算 Hash
	hashStr := helper.HashFeishuAppConfig(appid, appSecret)

	// 2. 局部锁：检查并提取旧客户端，然后立即更新/删除
	receiver.lock.Lock()
	oldClient, exists := receiver.clients[hashStr]
	// 这里先不创建新客户端，先处理旧的逻辑
	receiver.lock.Unlock()

	// 3. 在锁外执行销毁逻辑（避免阻塞其他客户端的操作）
	if exists && oldClient != nil {
		zap.L().Info("正在更新通道的客户端，关闭旧连接", zap.String("alertChannelName", alertChannelName))
		if oldClient.cancelFn != nil {
			oldClient.cancelFn() // 触发异步关闭
		}
	}

	// 4. 创建新的客户端（这个过程可能涉及网络请求获取 token，比较耗时，务必在锁外执行）
	cli, wsCli, cancel := newFeishuClient(alertChannelName, appid, appSecret)

	// 5. 重新加锁写入新客户端
	receiver.lock.Lock()
	receiver.clients[hashStr] = &Client{
		cli:      cli,
		wsCli:    wsCli,
		cancelFn: cancel,
	}
	clientNames := printFeishuCli() // 假设这个函数内部也是线程安全的
	receiver.lock.Unlock()

	zap.L().Info("已更新飞书客户端", zap.String("alertChannelName", alertChannelName), zap.String("clientNames", clientNames))
}

// UpdateCli 如果 appid 和 appSecret 修改需要重新初始化客户端
func (receiver *FeiShu) CloseCli(alertChannelName, appid, appSecret string) {
	hashStr := helper.HashFeishuAppConfig(appid, appSecret)
	receiver.lock.Lock()
	defer receiver.lock.Unlock()

	if oldClient, ok := receiver.clients[hashStr]; ok {
		zap.L().Info("正在关闭通道的客户端旧连接", zap.String("alertChannelName", alertChannelName))
		if oldClient.cancelFn != nil {
			oldClient.cancelFn()
			time.Sleep(5 * time.Second)
		}
	}
	delete(receiver.clients, hashStr)
	clientNames := printFeishuCli()
	zap.L().Info("从本地缓存中删除客户端成功", zap.String("alertChannelName", alertChannelName), zap.String("clientNames", clientNames))
}

func (receiver *FeiShu) renderAndSend(ctx context.Context, larkCli *lark.Client, receiveIdType, receiveId string, data interface{}, tpl string, color string) error {
	log.WithRequestID(ctx).Debug("发送飞书发送告警",
		zap.String("receiveIdType", receiveIdType),
		zap.String("receiveId", receiveId),
		zap.String("tpl", tpl),
	)

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
	return SendCard(larkCli).Build(ctx, receiveIdType, receiveId, string(byData))
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
		log.WithRequestID(ctx).Error(
			"发起请求发送飞书卡片时服务发生错误",
			zap.String("logId", resp.RequestId()),
			zap.String("codeError", larkcore.Prettify(resp.CodeError)),
			zap.Error(err),
		)
		reqID := fmt.Sprintf("requestID: %v", log.GetRequestIDFromContext(ctx))
		codeError := fmt.Sprintf("codeError: %v", larkcore.Prettify(resp.CodeError))
		sendErr := fmt.Sprintf("err: %v", err)
		return fmt.Errorf("%v \n%v\n%v", reqID, codeError, sendErr)
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

func (receiver *FeishuCardDataContent) Build(ctx context.Context, alert any, alertTpl string) (*FeiShuContent, error) {
	tmpl, err := template.New("alert").Funcs(helper.FuncMap).Parse(alertTpl)
	if err != nil {
		return nil, fmt.Errorf("构建告警模版失败, %s", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, alert); err != nil {
		return nil, fmt.Errorf("渲染告警模版失败, %s", err)
	}

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
	larkCli, err := receiver.GetCli(alertChannel.Name, feishuAppConf.AppID, feishuAppConf.AppSecret)
	if err != nil {
		return nil, err
	}

	alertArry := notifyReq.AlertArry
	// 聚合发送告警
	if *notifyReq.AlertChannel.AggregationStatus == model.AggregationEnabled {
		log.WithRequestID(ctx).Debug("聚合发送告警")
		receiveIds := notifyReq.AlertTemplate.ReceiveId
		if len(alertArry.FiringAlertArry) > 0 {
			newReq := notifyReq.AlertReceiveReq.DeepCopy()
			newReq.Alerts = alertArry.FiringAlertArry
			for _, rid := range receiveIds {
				// 重写 at 人
				_rid, _template := helper.OverrideAt(rid, notifyReq.AlertTemplate.AggregationTemplate)
				err = receiver.renderAndSend(
					ctx,
					larkCli,
					notifyReq.AlertTemplate.ReceiveIdType,
					_rid,
					newReq,
					_template,
					"red",
				)
				if err != nil {
					firingErr = err
				}
			}
		}

		if len(alertArry.ResolvedAlertArry) > 0 {
			newReq := notifyReq.AlertReceiveReq.DeepCopy()
			newReq.Alerts = alertArry.ResolvedAlertArry
			for _, rid := range receiveIds {
				// 重写 at 人
				_rid, _template := helper.OverrideAt(rid, notifyReq.AlertTemplate.AggregationTemplate)
				err = receiver.renderAndSend(
					ctx,
					larkCli,
					notifyReq.AlertTemplate.ReceiveIdType,
					_rid,
					newReq,
					_template,
					"green",
				)
				if err != nil {
					resolvedErr = err
				}
			}
		}

		return &types.NotifySendResult{
			AggregationSendResult: &types.AggregationSendResult{
				FiringErr:   firingErr,
				ResolvedErr: resolvedErr,
			},
			SingleSendResult: nil,
		}, err
	}

	if *notifyReq.AlertChannel.AggregationStatus == model.AggregationDisabled {
		// 非聚合发送
		receiveIdType := notifyReq.AlertTemplate.ReceiveIdType
		receiveIds := notifyReq.AlertTemplate.ReceiveId
		normalSendResult, err := receiver.singleSend(ctx, larkCli, receiveIdType, receiveIds, notifyReq.AlertTemplate, alertArry)
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

func (receiver *FeiShu) singleSend(ctx context.Context, larkCli *lark.Client, receiveIdType string, receiveIds []string, alertTemplate *model.AlertTemplate, alertArry *types.AlertArry) ([]*types.SingleSendResult, error) {
	var (
		errs    []error
		results []*types.SingleSendResult
	)

	for _, rid := range receiveIds {
		if rid == "" {
			continue
		}
		process := func(v *types.Alert) {
			color := "red"
			if v.Status == constant.AlertStatusResolved {
				color = "green"
			}

			// 重写 at 人
			_rid, _template := helper.OverrideAt(rid, alertTemplate.Template)

			err := receiver.renderAndSend(ctx, larkCli, receiveIdType, _rid, v, _template, color)
			results = append(results, &types.SingleSendResult{
				Alert:   v,
				SendErr: err,
			})

			if err != nil {
				log.WithRequestID(ctx).Error("发送单条飞书卡片失败", zap.Error(err))
				// 限制错误收集数量
				if len(errs) < 4 {
					errs = append(errs, err)
				}
			}
			// 防止飞书限流
			time.Sleep(time.Microsecond * 200)
		}
		for _, v := range alertArry.FiringAlertArry {
			process(v)
		}
		for _, v := range alertArry.ResolvedAlertArry {
			process(v)
		}
	}

	return results, errors.Join(errs...)
}
