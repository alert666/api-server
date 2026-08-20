package alert_test

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alert666/api-server/base/config"
	"github.com/alert666/api-server/base/data"
	"github.com/alert666/api-server/base/helper"
	"github.com/alert666/api-server/base/log"
	"github.com/alert666/api-server/base/types"
	"github.com/alert666/api-server/model"
	"github.com/alert666/api-server/pkg/alertinhibit"
	"github.com/alert666/api-server/pkg/email"
	"github.com/alert666/api-server/pkg/feishu"
	v1 "github.com/alert666/api-server/service/v1"
	"github.com/alert666/api-server/store"
	"go.uber.org/zap"
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
	tpl := `
{{- if .Alerts -}}
{{- $first := index .Alerts 0 -}}
{{- $count := len .Alerts -}}
{{- /* 先在开头计算好所有变量 */ -}}
{{- $fullDesc := "" -}}
{{- range $i, $v := .Alerts -}}
  {{- if lt $i 3 -}}
    {{- $line := printf "%d. %s\n" (add $i 1) (index $v.Annotations "description") -}}
    {{- $fullDesc = printf "%s%s" $fullDesc $line -}}
  {{- end -}}
{{- end -}}
{{- if gt $count 3 -}}
  {{- $footer := printf "---\n💡 当前已聚合 %d 条告警，仅展示前 3 条。" $count -}}
  {{- $fullDesc = printf "%s%s" $fullDesc $footer -}}
{{- end -}}

{{- /* YAML 输出结构 */ -}}
template_id: "AAqK947a7l70i"
template_version_name: "1.0.10"
template_variable:
  alertName: {{ if gt $count 1 }}{{ printf "[聚合%d条告警] %s" $count (index $first.Labels "alertname") | printf "%q" }}{{ else }}{{ index $first.Labels "alertname" | printf "%q" }}{{ end }}
  alertCluster: {{ index $first.Labels "cluster" | printf "%q" }}
  alertLevel: {{ index $first.Labels "severity" | printf "%q" }}
  alertStartTime: {{ timeFormat $first.StartsAt | printf "%q" }}
  alertEndTime: {{ getEndTime $first.EndsAt "告警未恢复" | printf "%q" }}
  alertUser: "<at id=28c4bfgf></at>"
  disableSelect: false
  alertDescribe: {{ $fullDesc | printf "%q" }}
  {{- /* ⚠️ 注意：grafanaLink 必须保持原始 JSON 对象格式，不要加 printf "%q" */}}
  grafanaLink: {{ newViewLink (getGrafanaExploreLink "https://kp-grafana.prod.karmada.suanleme.local" $first.GeneratorURL "thanos" ) }}
{{- end -}}`

	data := `{"receiver":"prometheusalert","status":"firing","alerts":[{"status":"firing","labels":{"alertname":"4583PodNotRunning","area":"guangdong","belong":"idc","component":"kube-state-metrics","container":"kube-rbac-proxy-main","index":"01","instance":"172.20.91.44:8443","job":"kube-state-metrics","namespace":"jb8ppchug27a2uhoyre7efcfbok2w0ct-4583","phase":"Pending","pod":"deployment-4583-dhnhlbyg-56469f6fc8-7vxqv","prometheus":"monitoring/k8s","provider":"guangdong","range":"cluster","severity":"critical","type":"prod","uid":"f4203d26-ba2b-4133-9de8-b80d07e5f058"},"annotations":{"description":"Pod deployment-4583-dhnhlbyg-56469f6fc8-7vxqv in namespace test is in Pending state.","summary":"Pod not running in test namespace"},"startsAt":"2026-04-01T02:15:46.098Z","endsAt":"0001-01-01T00:00:00Z","generatorURL":"http://prometheus-k8s-0:9090/graph?g0.expr=kube_pod_status_phase%7Bnamespace%3D%22jb8ppchug27a2uhoyre7efcfbok2w0ct-4583%22%2Cphase%21~%22Running%7CSucceeded%22%7D+%3D%3D+1\u0026g0.tab=1","fingerprint":"060c3ec7f26a12a2"},{"status":"firing","labels":{"alertname":"4583PodNotRunning","area":"guangdong","belong":"idc","component":"kube-state-metrics","container":"kube-rbac-proxy-main","index":"01","instance":"172.20.91.44:8443","job":"kube-state-metrics","namespace":"jb8ppchug27a2uhoyre7efcfbok2w0ct-4583","phase":"Pending","pod":"deployment-4583-dhnhlbyg-56469f6fc8-cgldb","prometheus":"monitoring/k8s","provider":"guangdong","range":"cluster","severity":"critical","type":"prod","uid":"1f68e381-b918-45e2-9e71-60af28a62b1f"},"annotations":{"description":"Pod deployment-4583-dhnhlbyg-56469f6fc8-cgldb in namespace test is in Pending state.","summary":"Pod not running in test namespace"},"startsAt":"2026-04-01T02:15:16.098Z","endsAt":"0001-01-01T00:00:00Z","generatorURL":"http://prometheus-k8s-0:9090/graph?g0.expr=kube_pod_status_phase%7Bnamespace%3D%22jb8ppchug27a2uhoyre7efcfbok2w0ct-4583%22%2Cphase%21~%22Running%7CSucceeded%22%7D+%3D%3D+1\u0026g0.tab=1","fingerprint":"2965023e26da6136"},{"status":"firing","labels":{"alertname":"4583PodNotRunning","area":"guangdong","belong":"idc","component":"kube-state-metrics","container":"kube-rbac-proxy-main","index":"01","instance":"172.20.91.44:8443","job":"kube-state-metrics","namespace":"jb8ppchug27a2uhoyre7efcfbok2w0ct-4583","phase":"Pending","pod":"deployment-4583-dhnhlbyg-56469f6fc8-w7njg","prometheus":"monitoring/k8s","provider":"guangdong","range":"cluster","severity":"critical","type":"prod","uid":"189f785d-22f7-4656-8e2e-044e0e35c664"},"annotations":{"description":"Pod deployment-4583-dhnhlbyg-56469f6fc8-w7njg in namespace test is in Pending state.","summary":"Pod not running in test namespace"},"startsAt":"2026-04-01T02:15:46.098Z","endsAt":"0001-01-01T00:00:00Z","generatorURL":"http://prometheus-k8s-0:9090/graph?g0.expr=kube_pod_status_phase%7Bnamespace%3D%22jb8ppchug27a2uhoyre7efcfbok2w0ct-4583%22%2Cphase%21~%22Running%7CSucceeded%22%7D+%3D%3D+1\u0026g0.tab=1","fingerprint":"6a0200d57d18060d"}],"groupLabels":{"alertname":"4583PodNotRunning","namespace":"jb8ppchug27a2uhoyre7efcfbok2w0ct-4583"},"commonLabels":{"alertname":"4583PodNotRunning","area":"guangdong","belong":"idc","component":"kube-state-metrics","container":"kube-rbac-proxy-main","index":"01","instance":"172.20.91.44:8443","job":"kube-state-metrics","namespace":"jb8ppchug27a2uhoyre7efcfbok2w0ct-4583","phase":"Pending","prometheus":"monitoring/k8s","provider":"guangdong","range":"cluster","severity":"critical","type":"prod"},"commonAnnotations":{"summary":"Pod not running in test namespace"},"externalURL":"http://alertmanager-main-0:9093","version":"4","groupKey":"{}/{severity=\"critical\"}:{alertname=\"4583PodNotRunning\", namespace=\"jb8ppchug27a2uhoyre7efcfbok2w0ct-4583\"}","truncatedAlerts":0}
`

	req := &feishu.FeishuCardDataContent{}
	var payload *types.AlertReceiveReq
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		t.Fatal(err)
	}

	content, err := req.Build(context.TODO(), payload, tpl)
	if err != nil {
		t.Fatal(err)
	}

	by, err := json.Marshal(&content)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(by))
}

// appendClusterToPromQL 纯标准库实现：精准识别 PromQL 中的大括号并安全注入动态 cluster 标签
func appendClusterToPromQL(promQL, cluster string) string {
	if cluster == "" {
		return promQL
	}

	var result strings.Builder
	remaining := promQL

	// 从配置中动态获取集群/租户的 Key 名（例如 "cluster"、"tenant_id"）
	clusterKey := config.GetAlertTenantKey()
	targetLabel := fmt.Sprintf(`%s=%q`, clusterKey, cluster)

	for {
		// 1. 定位大括号的起始位置
		start := strings.Index(remaining, "{")
		if start == -1 {
			result.WriteString(remaining)
			break
		}
		// 把大括号前的内容（指标名、操作符等）原样写入
		result.WriteString(remaining[:start])
		remaining = remaining[start:]

		// 2. 定位闭合大括号
		end := strings.Index(remaining, "}")
		if end == -1 {
			result.WriteString(remaining)
			break
		}

		// 提取出大括号内部的标签文本
		inside := remaining[1:end]
		remaining = remaining[end+1:]

		// 3. 智能解析并重构内部标签
		var newLabels []string

		if len(strings.TrimSpace(inside)) > 0 {
			parts := strings.Split(inside, ",")
			hasCluster := false

			for _, part := range parts {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}

				// =================== 核心修复逻辑 ===================
				// 寻找 PromQL 标签中可能出现的四种比较符：=~, !~, =, !=
				idx := -1
				if i := strings.Index(part, "=~"); i != -1 {
					idx = i
				}
				if i := strings.Index(part, "!~"); i != -1 && (idx == -1 || i < idx) {
					idx = i
				}
				if i := strings.Index(part, "!="); i != -1 && (idx == -1 || i < idx) {
					idx = i
				}
				if i := strings.Index(part, "="); i != -1 && (idx == -1 || i < idx) {
					idx = i
				}

				// 如果找到了比较符，切出前面的 Key 名字进行【精准比对】
				if idx != -1 {
					currentKey := strings.TrimSpace(part[:idx])
					if currentKey == clusterKey {
						// 名字完全一致，执行替换
						newLabels = append(newLabels, targetLabel)
						hasCluster = true
						continue
					}
				}

				// 如果找到了比较符，切出前面的 Key 名字进行【精准比对】
				if idx != -1 {
					currentKey := strings.TrimSpace(part[:idx])
					if currentKey == clusterKey {
						// 名字完全一致，执行替换
						newLabels = append(newLabels, targetLabel)
						hasCluster = true
						continue
					}
				}
				// ===================================================

				// 不是我们要找的 clusterKey，原样保留
				newLabels = append(newLabels, part)
			}

			// 如果循环结束发现原本没有该标签，则追加
			if !hasCluster {
				newLabels = append(newLabels, targetLabel)
			}
		} else {
			// 原本是大括号为空 `{}` 的情况
			newLabels = append(newLabels, targetLabel)
		}

		// 4. 将重构后的标签重新组合并高效写入结果（消灭 Inefficient string concatenation 警告）
		result.WriteByte('{')
		result.WriteString(strings.Join(newLabels, ","))
		result.WriteByte('}')
	}

	return result.String()
}

func TestParserUrl(t *testing.T) {
	// 1. 定义匿名函数逻辑
	generateGrafanaURL := func(grafanaAddr, cluster, genURL, datasource string) string {
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

		if cluster != "" {
			promQL = appendClusterToPromQL(promQL, cluster)
		}

		fmt.Println("promQL", promQL)

		stateJSON := fmt.Sprintf(
			`{"datasource":%q,"queries":[{"expr":%q,"refId":"A"}],"range":{"from":"now-1h","to":"now"}}`,
			datasource,
			promQL,
		)

		// 3. 拼接并返回
		return grafanaAddr + "/explore?left=" + url.QueryEscape(stateJSON)
	}

	req := types.NewTestAlertReceiveReq()
	cluster := req.GroupLabels["cluster"]
	// cluster := ""
	// 2. 测试调用
	grafanaBase := "https://kp-grafana.prod.karmada.suanleme.local"
	prometheusGenURL := `http://prometheus-k8s-0:9090/graph?g0.expr=%28%28kube_node_spec_taint%7Bjob%3D%22kube-state-metrics%22%7D+unless+on+%28node%2C+key%2C+value%2C+effect%29+kube_node_spec_taint%7Bjob%3D%22kube-state-metrics%22%2Ckey%3D%22node.kubernetes.io%2Ftencent-cloud-node-termination%22%7D%29+unless+on+%28node%29+%28kube_node_labels%7Blabel_gongjiyun_com_ignore_unschedule%3D%22true%22%7D%29%29+%2A+on+%28node%29+group_left+%28internal_ip%2C+hostname%29+max+by+%28node%2C+internal_ip%2C+hostname%29+%28kube_node_info%7Bjob%3D%22kube-state-metrics%22%7D%29&g0.tab=1`

	finalURL := generateGrafanaURL(grafanaBase, cluster, prometheusGenURL, "thanos")
	fmt.Println("生成的地址:")
	fmt.Println(finalURL)
}

func TestAppendClusterToPromQL_Scenarios(t *testing.T) {
	clusterKey := "cluster" // 假设 conf.GetAlertTenantKey() 返回 "cluster"
	targetCluster := "karmada-cluster-01"

	tests := []struct {
		name        string
		rawURL      string
		wantContain []string // 期待修改后的 PromQL 包含这些特征
		wantExclude []string // 期待修改后的 PromQL 绝对不能包含这些特征
	}{
		{
			name:   "场景 1：标准覆盖测试（原 PromQL 没有任何 cluster 标签，期待全部追加）",
			rawURL: `http://prometheus-k8s-0:9090/graph?g0.expr=%28%28kube_node_spec_taint%7Bjob%3D%22kube-state-metrics%22%7D+unless+on+%28node%2C+key%2C+value%2C+effect%29+kube_node_spec_taint%7Bjob%3D%22kube-state-metrics%22%2Ckey%3D%22node.kubernetes.io%2Ftencent-cloud-node-termination%22%7D%29+unless+on+%28node%29+%28kube_node_labels%7Blabel_gongjiyun_com_ignore_unschedule%3D%22true%22%7D%29%29+%2A+on+%28node%29+group_left+%28internal_ip%2C+hostname%29+max+by+%28node%2C+internal_ip%2C+hostname%29+%28kube_node_info%7Bjob%3D%22kube-state-metrics%22%7D%29`,
			wantContain: []string{
				`kube_node_spec_taint{job="kube-state-metrics",cluster="karmada-cluster-01"}`,
				`kube_node_info{job="kube-state-metrics",cluster="karmada-cluster-01"}`,
			},
		},
		{
			name:   "场景 2：防止误伤测试（原 PromQL 带有 cluster_id 和 cluster_name，期待保留它们并追加 cluster）",
			rawURL: `http://prometheus-k8s-0:9090/graph?g0.expr=kube_node_info%7Bjob%3D%22ksm%22%2Ccluster_id%3D%22123%22%2Ccluster_name%3D%22prod%22%7D`,
			wantContain: []string{
				`cluster_id="123"`,
				`cluster_name="prod"`,
				`cluster="karmada-cluster-01"`, // 正确追加了 cluster
			},
		},
		{
			name:   "场景 3：旧值强擦覆盖测试（原 PromQL 已经带了旧的 cluster='old'，期待将其替换为新值）",
			rawURL: `http://prometheus-k8s-0:9090/graph?g0.expr=kube_node_info%7Bjob%3D%22ksm%22%2Ccluster%3D%22old-cluster-value%22%7D`,
			wantContain: []string{
				`cluster="karmada-cluster-01"`, // 替换成功
			},
			wantExclude: []string{
				`old-cluster-value`, // 旧值必须消失
			},
		},
		{
			name:   "场景 4：复杂操作符测试（原 PromQL 包含了正则匹配 =~ 和 不等于 != 的 cluster 标签，期待全部强制覆盖）",
			rawURL: `http://prometheus-k8s-0:9090/graph?g0.expr=kube_node_info%7Bcluster%3D~%22karmada-.*%22%7D+unless+kube_node_labels%7Bcluster%21%3D%22offline%22%7D`,
			wantContain: []string{
				`kube_node_info{cluster="karmada-cluster-01"}`,
				`kube_node_labels{cluster="karmada-cluster-01"}`,
			},
			wantExclude: []string{
				`cluster=~`,
				`cluster!=`,
			},
		},
		{
			name:   "场景 5：空大括号异常测试（原 PromQL 带有空大括号对如 up{}，期待能正常塞入新标签）",
			rawURL: `http://prometheus-k8s-0:9090/graph?g0.expr=up%7B%7D+%2A+kube_node_info%7B%7D`,
			wantContain: []string{
				`up{cluster="karmada-cluster-01"}`,
				`kube_node_info{cluster="karmada-cluster-01"}`,
			},
		},
	}

	fmt.Printf("======= 开始跑 PQL 注入算法压测（测试 Key: %s）=======\n\n", clusterKey)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 1. 解析 URL 拿到原始 PromQL
			u, err := url.Parse(tt.rawURL)
			if err != nil {
				t.Fatalf("测试数据构建失败，URL 无法解析: %v", err)
			}
			rawPromQL := u.Query().Get("g0.expr")

			// 2. 运行你的算法
			gotPromQL := appendClusterToPromQL(rawPromQL, targetCluster)

			// 3. 验证包含项
			for _, contain := range tt.wantContain {
				if !strings.Contains(gotPromQL, contain) {
					t.Errorf("\n【测试失败】: %s\n期待包含: %s\n实际输出: %s\n", tt.name, contain, gotPromQL)
					return
				}
			}

			// 4. 验证排除项
			for _, exclude := range tt.wantExclude {
				if strings.Contains(gotPromQL, exclude) {
					t.Errorf("\n【测试失败】: %s\n不应包含: %s\n实际输出: %s\n", tt.name, exclude, gotPromQL)
					return
				}
			}

			fmt.Printf("✅ 通过 -> %s\n", tt.name)
		})
	}
}

var testData = `{"ChannelName":"feishu","receiver":"prometheusalert","status":"resolved","alerts":[{"status":"resolved","labels":{"alertname":"DaemonSet滚动更新卡住","area":"chengde","belong":"idc","component":"daemonset","container":"kube-rbac-proxy-main","daemonset":"pod-mgr-igde-p-1-worker-10","failure_type":"rollout_stuck","impact_level":"high","index":"01","instance":"172.20.5.178:8443","job":"kube-state-metrics","namespace":"system","prometheus":"monitoring/k8s","provider":"chengde","range":"cluster","severity":"warning","type":"prod"},"annotations":{"description":"【Kubernetes守护进程集更新异常】\n命名空间: system\nDaemonSet名称: pod-mgr-igde-p-1-worker-10\n集群: \n\n当前状态:\n- 期望调度Pod数: 1\n- 实际调度Pod数: 1\n- 错误调度Pod数: 0\n- 已更新Pod数: 1\n- 可用Pod数: 0","runbook_url":"https://runbooks.prometheus-operator.dev/runbooks/kubernetes/kubedaemonsetrolloutstuck","summary":"DaemonSetpod-mgr-igde-p-1-worker-10 更新停滞 (命名空间: system)"},"startsAt":"2026-04-09T12:58:46.012Z","endsAt":"2026-04-09T13:14:16.012Z","generatorURL":"http://prometheus-k8s-0:9090/graph?g0.expr=...","fingerprint":"28c89c4f51cb8e24","isSilenced":false,"silenceID":0},{"status":"resolved","labels":{"alertname":"DaemonSet滚动更新卡住","area":"chengde","belong":"idc","component":"daemonset","container":"kube-rbac-proxy-main","daemonset":"pod-shutdown-operator","failure_type":"rollout_stuck","impact_level":"high","index":"01","instance":"172.20.5.178:8443","job":"kube-state-metrics","namespace":"system","prometheus":"monitoring/k8s","provider":"chengde","range":"cluster","severity":"warning","type":"prod"},"annotations":{"description":"【Kubernetes守护进程集更新异常】\n命名空间: system\nDaemonSet名称: pod-shutdown-operator\n集群: \n\n当前状态:\n- 期望调度Pod数: 4\n- 实际调度Pod数: 4\n- 错误调度Pod数: 0\n- 已更新Pod数: 4\n- 可用Pod数: 3","runbook_url":"https://runbooks.prometheus-operator.dev/runbooks/kubernetes/kubedaemonsetrolloutstuck","summary":"DaemonSet pod-shutdown-operator 更新停滞 (命名空间: system)"},"startsAt":"2026-04-09T12:59:16.012Z","endsAt":"2026-04-09T13:14:16.012Z","generatorURL":"http://prometheus-k8s-0:9090/graph?g0.expr=...","fingerprint":"f083a4a194dcd965","isSilenced":false,"silenceID":0}],"groupLabels":{"alertname":"DaemonSet滚动更新卡住","instance":"172.20.5.178:8443","namespace":"system"},"commonLabels":{"alertname":"DaemonSet滚动更新卡住","area":"chengde","belong":"idc","component":"daemonset","container":"kube-rbac-proxy-main","failure_type":"rollout_stuck","impact_level":"high","index":"01","instance":"172.20.5.178:8443","job":"kube-state-metrics","namespace":"system","prometheus":"monitoring/k8s","provider":"chengde","range":"cluster","severity":"warning","type":"prod"},"commonAnnotations":{"runbook_url":"https://runbooks.prometheus-operator.dev/runbooks/kubernetes/kubedaemonsetrolloutstuck"},"externalURL":"http://alertmanager-main-0:9093","version":"4","groupKey":"{}:{alertname=\"DaemonSet滚动更新卡住\", instance=\"172.20.5.178:8443\", namespace=\"system\"}","truncatedAlerts":0}`

func TestGetAlertDescript(t *testing.T) {
	var req types.AlertReceiveReq
	if err := json.Unmarshal([]byte(testData), &req); err != nil {
		t.Fatal(err)
	}

	// 等价于模板中的逻辑
	func1 := func(data any) string {
		switch d := data.(type) {
		case *types.Alert:
			// 单条告警，直接返回 description
			return d.Annotations["description"]

		case []*types.Alert:
			count := len(d)
			if count == 0 {
				return ""
			}

			var sb strings.Builder
			for i, v := range d {
				if i < 3 {
					// 对应模板: {{- $line := printf "%d. %s\n" (add $i 1) (index $v.Annotations "description") -}}
					sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, v.Annotations["description"]))
				} else {
					// 超过 3 条就可以提前结束循环了，优化性能
					break
				}
			}

			// 对应模板: {{- if gt $count 3 -}} ...
			if count > 3 {
				sb.WriteString(fmt.Sprintf("---\n💡 当前已聚合 %d 条告警，仅展示前 3 条。", count))
			}

			return sb.String()

		default:
			return ""
		}
	}

	// 1. 测试聚合告警逻辑 (传入[]*types.Alert)
	fmt.Println("========== 聚合告警测试 ==========")
	aggResult := func1(req.Alerts)
	fmt.Println(aggResult)

	// 2. 测试单条告警逻辑 (传入 *types.Alert)
	fmt.Println("========== 单条告警测试 ==========")
	singleResult := func1(req.Alerts[0])
	fmt.Println(singleResult)

	// 3. 模拟超过 3 条告警的测试
	fmt.Println("========== 超过3条聚合告警测试 ==========")
	mockLargeAlerts := append(req.Alerts, req.Alerts...) // 复制一份，变成 4 条
	largeAggResult := func1(mockLargeAlerts)
	fmt.Println(largeAggResult)
}

func TestIsSilenced(t *testing.T) {
	// var req *types.AlertReceiveReq
	// json.Unmarshal([]byte(testData), &req)

	// enable := 1
	// now := time.Now()
	// silences := make([]*model.AlertSilence, 0)

	// silences = append(silences, &model.AlertSilence{
	// 	ID:          1,
	// 	Cluster:     "chengde",
	// 	Type:        1,
	// 	Fingerprint: "28c89c4f51cb8e24",
	// 	Status:      &enable,
	// 	EndsAt:      now.Add(100 * time.Hour),
	// 	StartsAt:    now.Add(-100 * time.Hour),
	// })

	// matcher := &model.Matcher{
	// 	Name:  "alertname",
	// 	Type:  "=",
	// 	Value: "DaemonSet滚动更新卡住",
	// }
	// matchers := make([]*model.Matcher, 0)
	// matchers = append(matchers, matcher)

	// matchersBy, err := json.Marshal(&matchers)
	// if err != nil {
	// 	t.Fatal(err)
	// }

	// silences = append(silences, &model.AlertSilence{
	// 	ID:          2,
	// 	Cluster:     "chengde",
	// 	Type:        2,
	// 	Fingerprint: "28c89c4f51cb8e24",
	// 	Status:      &enable,
	// 	EndsAt:      now.Add(100 * time.Hour),
	// 	StartsAt:    now.Add(-100 * time.Hour),
	// 	Matchers:    matchersBy,
	// })

	// matcher1 := &model.Matcher{
	// 	Name:  "alertname",
	// 	Type:  "!=",
	// 	Value: "DaemonSet滚动更新卡住",
	// }
	// matchers1 := make([]*model.Matcher, 0)
	// matchers1 = append(matchers1, matcher1)

	// matchersBy1, err := json.Marshal(&matchers1)
	// if err != nil {
	// 	t.Fatal(err)
	// }

	// silences = append(silences, &model.AlertSilence{
	// 	ID:          3,
	// 	Cluster:     "chengde",
	// 	Type:        2,
	// 	Fingerprint: "28c89c4f51cb8e24",
	// 	Status:      &enable,
	// 	EndsAt:      now.Add(100 * time.Hour),
	// 	StartsAt:    now.Add(-100 * time.Hour),
	// 	Matchers:    matchersBy1,
	// })

	alertsServicer, err := v1.NewAlertsServicer(nil, nil, email.Emailer(nil))
	if err != nil {
		t.Fatal(err)
	}
	err = config.LoadConfig("../../config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	log.NewLogger()
	data.NewDB()
	var activeSilences []*model.AlertSilence
	now := time.Now()
	err = store.AlertSilence.WithContext(context.TODO()).
		UnderlyingDB().
		Where("cluster = ?", "xinjiang").
		Where(store.AlertSilence.Status.Eq(model.SilenceEnabled)).
		Where(store.AlertSilence.EndsAt.Gte(now)).
		Where(store.AlertSilence.StartsAt.Lte(now)).
		Find(&activeSilences).Error
	if err != nil {
		zap.L().Error("查询静默规则失败", zap.Error(err))
	}

	layout := "2006-01-02 15:04:05.000"

	startTime, err := time.Parse(layout, "2026-05-12 17:59:38.889")
	if err != nil {
		panic(err)
	}

	fmt.Println(t)

	alerts := make([]*types.Alert, 0)
	alerts = append(alerts, &types.Alert{
		Status:      "firing",
		StartsAt:    startTime,
		EndsAt:      nil,
		Fingerprint: "ab0c89768a745dcc",
	})
	// req := &types.AlertReceiveReq{
	// 	ChannelName: "idc",
	// 	Status:      "firing",
	// 	Alerts:
	// }

	for _, v := range alerts {
		silience, id := alertsServicer.IsSilenced(context.Background(), v, activeSilences)

		fmt.Println("☀️------------------------------------☀️")
		fmt.Println("silience", silience)
		fmt.Println("id", id)
		fmt.Println("🌙------------------------------------🌙")
	}

}

func TestGetData(t *testing.T) {
	cst, err := time.LoadLocation("Asia/Shanghai")
	time.Local = cst
	s := "2026-04-06T20:30:00+08:00"
	e := "2026-06-30T22:30:00+08:00"

	st, _ := time.Parse(time.RFC3339, s)
	en, _ := time.Parse(time.RFC3339, e)

	t1 := "2026-04-07 17:26:33.566"
	t2 := "2026-04-07 17:44:04"

	t1t, err := time.ParseInLocation(time.DateTime, t1, cst)
	if err != nil {
		t.Fatal(err)
	}

	t2t, err := time.ParseInLocation(time.DateTime, t2, cst)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("☀️------------------------------------☀️")
	fmt.Println(st.Unix())
	fmt.Println(en.Unix())
	fmt.Println(t1t.Unix())
	fmt.Println(t2t.Unix())
	fmt.Println("🌙------------------------------------🌙")
}

func AllMatchersMatch(matchers []*model.Matcher, alertLabels map[string]string) bool {
	for _, m := range matchers {
		val := alertLabels[m.Name]
		if !m.Matches(val) {
			return false
		}
	}
	return true
}

func TestAlertInhibitRulesConfig(t *testing.T) {
	err := config.LoadConfig("../../config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	log.NewLogger()

	client, err := data.NewRDB()
	if err != nil {
		t.Fatal(err)
	}
	cacheStore, cleanup, err := store.NewCacheStore(client)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	_, cleanup2, err := data.NewDB()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup2()

	v1.NewStore()
	matchers, err := alertinhibit.NewMatchers()
	if err != nil {
		t.Fatal(err)
	}

	inhibitImpl := v1.NewalertInhibit(matchers, cacheStore)
	inhibitImpl.CleanInhibitAlert()
}

func TestIntAddress(t *testing.T) {

	var a *int

	b := a

	fmt.Println(b)

	fmt.Println(config.GetOutboundIP())
}

const template = `template_id: "AAqtHMkdgJ5i6"
template_version_name: "1.0.1"
template_variable:
  alertName: {{ index .Labels "alertname" | printf "%q" }}
  alertDescribe: {{ getDescript . | printf "%q" }}
  alertCluster: {{ getClusterLabel (index .Labels "cluster") }}
  alertLevel: {{ index .Labels "severity" | printf "%q" }}
  alertStartTime: {{ timeFormat .StartsAt | printf "%q" }}
  alertEndTime: {{ getEndTime .EndsAt "告警未恢复" | printf "%q" }}
  alertUser: "<at id=ljh202606></at>"
  grafanaLink: {{ newViewLink (getGrafanaExploreLink "https://kp-grafana.prod.karmada.suanleme.local" .GeneratorURL "thanos" ) }}
  alertmanagerAddr: {{ newAlertManagerLink "https://cloud.suanlene.cn/workspace/alert/history?page=1&pageSize=15&status=firing" (index .Labels "cluster") }}`

func TestRese(t *testing.T) {
	rid, result := helper.OverrideAt("oc_119e4c05afe7189a9c82e52489ede217;;<at email=huyf@suanleme.cn></at>", template)
	fmt.Println("\n========== 🔥🔥🔥 DEBUG TRACE 🔥🔥🔥 ==========")
	fmt.Printf("[DBG] %s:%d %s = %#v\n", "regexp_test.go", 24, "rid", rid)
	fmt.Printf("[DBG] %s:%d %s = %#v\n", "regexp_test.go", 24, "result", result)
	fmt.Println("========== 🔥🔥🔥 DEBUG END 🔥🔥🔥 ==========")
}

func TestGetRemoteReceive(t *testing.T) {
	log.NewLogger()

	te := &model.AlertTemplate{
		ReceiveIdType: string(model.ReceiveIdTypeRemote),
		ReceiveId:     []string{"http://127.0.0.1:9090/api/v1/tenant/tenantPodRegion;;4045d6c1da2ab78e2fc21e6956bb79f4a5678b75d09d2eddaa8f838399043969;;chat_id"},
		// ReceiveId:     []string{"https://gongjiyun-business-data.suanlene.cn/api/v1/tenant/node-pod-region;;4045d6c1da2ab78e2fc21e6956bb79f4a5678b75d09d2eddaa8f838399043969;;chat_id"},
	}

	req := types.NewTestAlertReceiveReq()

	getReq := &types.RemoteReceiveReq{
		Cluster:         "tke-gateway",
		AlertReceiveReq: req,
		AlertTemplate:   te,
	}

	if err := helper.GetRemoteReceive(context.Background(), getReq); err != nil {
		t.Fatal(err)
	}

	by, err := json.Marshal(&getReq)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("\n========== 🔥🔥🔥 DEBUG TRACE 🔥🔥🔥 ==========")
	fmt.Println(string(by))
	fmt.Println("========== 🔥🔥🔥 DEBUG END 🔥🔥🔥 ==========")
}

func TestIDCCRon(t *testing.T) {
	if err := config.LoadConfig("../../config.yaml"); err != nil {
		t.Fatal(err)
	}

	log.NewLogger()

	_, c, err := data.NewDB()
	if err != nil {
		t.Fatal(err)
	}
	defer c()

	client, err := data.NewRDB()
	if err != nil {
		t.Fatal(err)
	}
	cacheStore, cleanup, err := store.NewCacheStore(client)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	feishuer := feishu.NewFeiShu()

	feishuer.Init("feishu", "xxx", "xxx")

	emailer := email.NewEmailSender()
	alertsServicer, err := v1.NewAlertsServicer(cacheStore, feishuer, emailer)

	if err != nil {
		t.Fatal(err)
	}

	v1.NewStore()
	idc := v1.NewIDCHeartbeat(cacheStore, alertsServicer)
	// idc.CronJobIDCHeartbeat()
	idc.CronJobIDCResolvedHeartbeat()
}
