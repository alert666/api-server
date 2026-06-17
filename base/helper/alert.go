package helper

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"text/template"
	"time"

	"github.com/alert666/api-server/base/log"
	"github.com/alert666/api-server/base/types"
	"github.com/alert666/api-server/model"
	"github.com/alert666/api-server/store"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

// HashFeishuAppConfig 对 飞书 配置进行 hash
func HashFeishuAppConfig(appid, appSecret string) string {
	h := sha256.New()
	h.Write([]byte(appid))
	h.Write([]byte(":"))
	h.Write([]byte(appSecret))
	return hex.EncodeToString(h.Sum(nil))
}

// ValidateTemplateRecipient 校验模板接收者配置
func ValidateTemplateRecipient(receiveIdType, receiveId string) error {
	switch receiveIdType {
	case "open_id", "user_id", "email", "chat_id":
		if receiveId == "" {
			return fmt.Errorf("接收者类型为 %s 时, receiveId 不能为空", receiveIdType)
		}
		return nil
	default:
		return fmt.Errorf("不支持的接收者类型: %s", receiveIdType)
	}
}

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
		if appID == nil {
			return fmt.Errorf("alertChannel.Config 飞书应用 ID 不存在")
		}
		if appSecret == nil {
			return fmt.Errorf("alertChannel.Config 飞书应用 secret 不存在")
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
	"getClusterLabel": func(cluster string) string {
		return store.GetTenantLabel(cluster)
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
	// 当告警Channel为飞书的时候, 设置飞书卡片按钮跳转 grafana 链接
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
	// 当告警Channel为飞书的时候, 设置飞书卡片按钮跳转平台的链接
	"newAlertManagerLink": func(link string, area string) string {
		u, err := url.Parse(link)
		if err != nil {
			return "{}"
		}
		params := u.Query()
		params.Add("tenant", area)
		u.RawQuery = params.Encode()

		m := map[string]string{
			"pc_url":      u.String(),
			"android_url": "",
			"ios_url":     "",
			"url":         u.String(),
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
			desc := d.Annotations["description"]
			if desc == "" {
				return "_无详细描述_"
			}
			// return fmt.Sprintf("```yaml\n%s\n```", strings.TrimSpace(desc))
			return strings.TrimSpace(desc)

		case []*types.Alert:
			count := len(d)
			if count == 0 {
				return ""
			}

			var sb strings.Builder
			for i, v := range d {
				if i >= 3 {
					break
				}

				sb.WriteString(fmt.Sprintf("<font color='red'>**告警实例 #%d**\n</font>", i+1))
				desc := v.Annotations["description"]
				// sb.WriteString("```yaml\n")
				sb.WriteString(strings.TrimSpace(desc))
				sb.WriteString("\n\n")
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
