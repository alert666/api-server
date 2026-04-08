package main

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
	"github.com/alert666/api-server/base/helper"
)

func TestCard(t *testing.T) {
	// // 创建 API Client
	// client := lark.NewClient("cli_a9348075d4f8dcc7", "c9eSiKuHzfPp2G7f0SLNhbasSHuyIetp",
	// 	// 开启调试日志（开发阶段很有用）
	// 	lark.WithLogLevel(larkcore.LogLevelDebug),
	// 	// 设置请求超时时间
	// 	lark.WithReqTimeout(10*time.Second),
	// 	// 如果是企业自建应用，可以配置以下（可选）
	// 	// lark.WithHelpdeskCredential("ID", "Token"),
	// )
	// 注册回调
	eventHandler := dispatcher.NewEventDispatcher("", "").
		OnP2CardActionTrigger(func(ctx context.Context, event *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error) {
			// fmt.Printf("[ OnP2CardActionTrigger access ], data: %s\n", larkcore.Prettify(event))
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
			return &callback.CardActionTriggerResponse{
				Toast: &callback.Toast{
					Type:    "info",
					Content: "静默成功",
				},
				Card: &callback.Card{
					Type: "template",
					Data: map[string]any{
						"template_id": "AAqK947a7l70i",
						"template_variable": map[string]any{
							"disableSelect": true,
						},
					},
				},
			}, nil
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
	cli := larkws.NewClient("xxx", "xxx",
		larkws.WithEventHandler(eventHandler),
		larkws.WithLogLevel(larkcore.LogLevelDebug),
	)
	// 建立长连接
	err := cli.Start(context.Background())
	if err != nil {
		panic(err)
	}
}
