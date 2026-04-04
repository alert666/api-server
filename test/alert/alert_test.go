package alert_test

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/qinquanliuxiang666/alertmanager/pkg/feishu"
	"gopkg.in/yaml.v2"
)

// AlertmanagerPayload 对应你提供的 JSON 结构
type AlertmanagerPayload struct {
	Alerts            []Alert           `json:"alerts"`
	CommonAnnotations map[string]string `json:"commonAnnotations"`
	CommonLabels      map[string]string `json:"commonLabels"`
	ExternalURL       string            `json:"externalURL"`
	GroupKey          string            `json:"groupKey"`
	GroupLabels       map[string]string `json:"groupLabels"`
	Receiver          string            `json:"receiver"`
	Status            string            `json:"status"`
	TruncatedAlerts   int               `json:"truncatedAlerts"`
	Version           string            `json:"version"`
}

type Alert struct {
	Annotations  map[string]string `json:"annotations"`
	EndsAt       time.Time         `json:"endsAt"`
	Fingerprint  string            `json:"fingerprint"`
	GeneratorURL string            `json:"generatorURL"`
	Labels       map[string]string `json:"labels"`
	StartsAt     time.Time         `json:"startsAt"`
	Status       string            `json:"status"`
}

// 模拟预设的数据池
var (
	alertNames = []string{"NodeDiskUsageHigh", "CPUThrottlingHigh", "MemoryLeakDetected", "ServiceDown", "KubePodCrashLooping"}
	severities = []string{"critical", "warning", "info"}
	teams      = []string{"infrastructure", "backend", "devops", "dba"}
)

// GenerateRandomAlerts 生成模拟数据
// totalAlerts: 总告警条数
// numGroups: 分成多少个组发送（返回一个切片，每个元素代表一个分组的 Payload）
func GenerateRandomAlerts(totalAlerts int, numGroups int) []AlertmanagerPayload {
	rand.Seed(time.Now().UnixNano())

	if numGroups <= 0 {
		numGroups = 1
	}
	alertsPerGroup := totalAlerts / numGroups

	var payloads []AlertmanagerPayload
	usedFingerprints := make(map[string]bool)

	for i := 0; i < numGroups; i++ {
		// 确定当前组的告警量
		count := alertsPerGroup
		if i == numGroups-1 { // 最后一组补齐余数
			count = totalAlerts - (alertsPerGroup * (numGroups - 1))
		}

		alertName := alertNames[rand.Intn(len(alertNames))]
		groupLabels := map[string]string{
			"alertname": alertName,
			"cluster":   "prod-aliyun-01",
		}

		payload := AlertmanagerPayload{
			Status:            "firing",
			Receiver:          "feishu-receiver",
			ExternalURL:       "http://alertmanager.example.com",
			Version:           "4",
			GroupLabels:       groupLabels,
			GroupKey:          fmt.Sprintf("{}/{alertname=%q, cluster=\"prod-aliyun-01\"}", alertName),
			CommonLabels:      groupLabels,
			CommonAnnotations: make(map[string]string),
			Alerts:            []Alert{},
		}

		for j := 0; j < count; j++ {
			instance := fmt.Sprintf("10.0.0.%d:9100", rand.Intn(254))
			startsAt := time.Now().Add(time.Duration(-rand.Intn(10000)) * time.Second)

			// 生成唯一的 Fingerprint: 基于实例名和时间戳生成 MD5
			hasher := md5.New()
			hasher.Write([]byte(fmt.Sprintf("%s-%d-%d", instance, startsAt.Unix(), rand.Int63())))
			fp := hex.EncodeToString(hasher.Sum(nil))[:16]

			// 确保唯一性（简单防重）
			for usedFingerprints[fp+startsAt.String()] {
				fp = fp[1:] + "1"
			}
			usedFingerprints[fp+startsAt.String()] = true

			alert := Alert{
				Status:       "firing",
				Fingerprint:  fp,
				StartsAt:     startsAt,
				EndsAt:       time.Time{}, // 0001-01-01
				GeneratorURL: fmt.Sprintf("http://vmalert:8080/vmalert/alert?id=%s", fp),
				Labels: map[string]string{
					"alertname": alertName,
					"instance":  instance,
					"severity":  severities[rand.Intn(len(severities))],
					"team":      teams[rand.Intn(len(teams))],
					"job":       "node-exporter",
					"device":    "/dev/sda1",
				},
				Annotations: map[string]string{
					"summary":     fmt.Sprintf("告警触发: %s 在 %s", alertName, instance),
					"description": fmt.Sprintf("检测到当前值 %d%% 超过阈值", rand.Intn(50)+50),
				},
			}
			payload.Alerts = append(payload.Alerts, alert)
		}
		payloads = append(payloads, payload)
	}

	return payloads
}

func TestAlert(t *testing.T) {
	// 配置参数
	totalAlerts := 500
	numGroups := 2
	outputDir := "alerts_output"

	// 1. 创建输出目录
	err := os.MkdirAll(outputDir, 0755)
	if err != nil {
		fmt.Printf("创建目录失败: %v\n", err)
		return
	}

	// 2. 生成数据
	payloads := GenerateRandomAlerts(totalAlerts, numGroups)

	// 3. 循环写入文件
	for i, payload := range payloads {
		// 生成文件名: alert_group_1_2023...json
		fileName := fmt.Sprintf("group_%d_%s.json", i+1, payload.GroupLabels["alertname"])
		filePath := filepath.Join(outputDir, fileName)

		// 格式化 JSON
		fileData, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			fmt.Printf("JSON 转换失败: %v\n", err)
			continue
		}

		// 写入磁盘
		err = os.WriteFile(filePath, fileData, 0644)
		if err != nil {
			fmt.Printf("文件 %s 写入失败: %v\n", filePath, err)
		} else {
			fmt.Printf("成功写入文件: %s (包含 %d 条告警)\n", filePath, len(payload.Alerts))
		}
	}

	fmt.Printf("\n所有告警已生成到目录: %s/\n", outputDir)

}

func TestString(t *testing.T) {
	// 使用反引号 ` 来包裹原始字符串，避免在定义阶段产生转义冲突
	rawURL := `https://gr.qqlx.net/explore?left={\"datasource\":\"vm\",\"queries\":[{\"expr\":%22container_memory_working_set_bytes%7Bimage%21%3D%5C%22%5C%22%2C+image%21~%5C%22pause%5C%22%2C+pod%3D~%5C%22.%2B%5C%22%7D+%2F+1024+%2F+1024+%3E+300%5Cn%22,\"refId\":\"A\"}],\"range\":{\"from\":\"1775302540000\",\"to\":\"now\"}}`

	// 将所有的 \ 替换为空字符串
	cleanURL := strings.ReplaceAll(rawURL, "\\", "")

	fmt.Println("清理后的 URL:")
	fmt.Println(cleanURL)
}

func TestTemplate(t *testing.T) {
	// 使用反引号定义的字符串，注意里面的缩进必须全部是空格
	data := `
template_id: "AAqK947a7l70i"
template_version_name: "1.0.10"
template_variable:
  alertName: "[聚合3条告警] ContainerMemoryUsageHigh"
  alertCluster: "local"
  alertLevel: "critical"
  alertStartTime: "2026-04-04 19:24:00"
  alertEndTime: 告警未恢复
  alertUser: "<at id=28c4bfgf></at>"
  disableSelect: false
  alertDescribe: "1. Pod cilium-tb5wt (命名空间: kube-system) 的容器内存使用量已超过 300MB (当前值: 380.97 MB)\n2. Pod kube-apiserver-node0 (命名空间: kube-system) 的容器内存使用量已超过 300MB (当前值: 440.84 MB)\n3. Pod cilium-ck9db (命名空间: kube-system) 的容器内存使用量已超过 300MB (当前值: 382.69 MB)\n"
  grafanaAddress: "{\"pc_url\":\"https://gr.qqlx.net/explore?left={\\\"datasource\\\":\\\"vm\\\",\\\"queries\\\":[{\\\"expr\\\":%22container_memory_working_set_bytes%7Bimage%21%3D%5C%22%5C%22%2C+image%21~%5C%22pause%5C%22%2C+pod%3D~%5C%22.%2B%5C%22%7D+%2F+1024+%2F+1024+%3E+300%5Cn%22,\\\"refId\\\":\\\"A\\\"}],\\\"range\\\":{\\\"from\\\":\\\"1775302540000\\\",\\\"to\\\":\\\"now\\\"}}\",\"android_url\":\"\",\"ios_url\":\"\",\"url\":\"\"}"`

	req := &feishu.FeishuCardDataContent{}

	// 确保使用的是 gopkg.in/yaml.v3 库
	if err := yaml.Unmarshal([]byte(data), &req); err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%+v\n", req)
}
