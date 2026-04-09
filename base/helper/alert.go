package helper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"text/template"
	"time"

	"github.com/alert666/api-server/base/log"
	"github.com/alert666/api-server/base/types"
	"github.com/alert666/api-server/model"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

func VerificationAlertFeishuConfig(channel *model.AlertChannel) (appid, appSecret string, err error) {
	var ok bool
	config := make(map[string]any, 0)
	err = json.Unmarshal(channel.Config, &config)
	if err != nil {
		return "", "", fmt.Errorf("验证飞书客户端配置失败, %s", err)
	}
	if err := VerificationAlertConfig(channel.Name, model.ChannelTypeFeishuApp, config); err != nil {
		return "", "", fmt.Errorf("验证飞书客户端配置失败, %s", err)
	}
	_appID := config["app_id"]
	appid, ok = _appID.(string)
	if !ok {
		return "", "", fmt.Errorf("获取飞书 app_id 置失败, %s", err)
	}
	_appSecret := config["app_secret"]
	appSecret, ok = _appSecret.(string)
	if !ok {
		return "", "", fmt.Errorf("获取飞书 app_secret 置失败, %s", err)
	}
	return appid, appSecret, nil
}

func VerificationAlertConfig(channelName string, channelType model.ChannelType, config map[string]any) error {
	switch channelType {
	case model.ChannelTypeFeishuApp:
		appID := config["app_id"]
		appSecret := config["app_secret"]
		receiveId := config["receive_id"]
		receiveIdType := config["receive_id_type"]
		if appID == nil {
			return fmt.Errorf("alertChannel.Config 飞书应用 ID 不存在")
		}
		if appSecret == nil {
			return fmt.Errorf("alertChannel.Config 飞书应用 secret 不存在")
		}
		if receiveId == nil {
			return fmt.Errorf("alertChannel.Config 飞书应用 receiveId 不存在")
		}
		if receiveIdType == nil {
			return fmt.Errorf("alertChannel.Config 飞书应用 receiveIdType 不存在")
		}
		return nil
	default:
		return fmt.Errorf("%s 告警是不支持的告警类型 %s", channelName, channelType)
	}
}

func GetAlertMapKey(fingerprint string, startAt time.Time) string {
	return fmt.Sprintf("%s-%d", fingerprint, startAt.UnixNano())
}

var FuncMap = template.FuncMap{
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
	// 当告警源为 prometheus 时，生成 Grafana Explore 链接
	"getGrafanaExploreLink": func(grafanaAddr, genURL, datasource string) string {
		if genURL == "" {
			return grafanaAddr + "/explore"
		}

		u, err := url.Parse(genURL)
		if err != nil {
			return grafanaAddr
		}

		promQL := u.Query().Get("g0.expr")
		if promQL == "" {
			return grafanaAddr + "/explore"
		}
		stateJSON := fmt.Sprintf(
			`{"datasource":%q,"queries":[{"expr":%q,"refId":"A"}],"range":{"from":"now-1h","to":"now"}}`,
			datasource,
			promQL,
		)
		return grafanaAddr + "/explore?left=" + url.QueryEscape(stateJSON)
	},
	// 当告警Channel为飞书的时候, 设置飞书卡片按钮跳转链接
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
	"getDescript": func(data any) string {
		switch d := data.(type) {
		case *types.Alert:
			return d.Annotations["description"]
		case []*types.Alert:
			count := len(d)
			if count == 0 {
				return ""
			}

			var sb strings.Builder
			for i, v := range d {
				if i < 3 {
					sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, v.Annotations["description"]))
				} else {
					break
				}
			}

			if count > 3 {
				sb.WriteString(fmt.Sprintf("---\n💡 当前已聚合 %d 条告警，仅展示前 3 条。", count))
			}

			return sb.String()

		default:
			return ""
		}
	},
}

func ValidateYamlTemplate(ctx context.Context, aggregation bool, alertTpl string) error {
	req := types.NewTestAlertReceiveReq()
	tmpl, err := template.New("test").Funcs(FuncMap).Parse(alertTpl)
	if err != nil {
		return fmt.Errorf("构建告警模版失败, %s", err)
	}

	validateFunc := func(data any) error {
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return fmt.Errorf("渲染告警模版失败, %s", err)
		}

		log.WithRequestID(ctx).Debug("测试告警模板", zap.String("data", buf.String()))

		var testObj map[string]any
		if err := yaml.Unmarshal([]byte(buf.Bytes()), &testObj); err != nil {
			return fmt.Errorf("序列化 testObj 失败, %s", err)
		}

		return nil
	}

	if aggregation {
		return validateFunc(req)
	}

	for _, v := range req.Alerts {
		if err := validateFunc(v); err != nil {
			return err
		}
	}
	return nil
}
