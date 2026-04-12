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

	"github.com/alert666/api-server/base/types"
	"github.com/alert666/api-server/model"
	"github.com/alert666/api-server/pkg/feishu"
	v1 "github.com/alert666/api-server/service/v1"
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

func TestParserUrl(t *testing.T) {
	// 1. 定义匿名函数逻辑
	generateGrafanaURL := func(grafanaAddr, genURL, datasource string) string {
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
		// 3. 拼接并返回
		return grafanaAddr + "/explore?left=" + url.QueryEscape(stateJSON)
	}

	// 2. 测试调用
	grafanaBase := "https://kp-grafana.prod.karmada.suanleme.local"
	prometheusGenURL := `http://prometheus-k8s-0:9090/graph?g0.expr=kube_pod_status_phase%7Bnamespace%3D%22jb8ppchug27a2uhoyre7efcfbok2w0ct-4583%22%2Cphase%21~%22Running%7CSucceeded%22%7D+%3D%3D+1&g0.tab=1`

	finalURL := generateGrafanaURL(grafanaBase, prometheusGenURL, "thanos")

	fmt.Println("生成的地址:")
	fmt.Println(finalURL)
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
	var req *types.AlertReceiveReq
	json.Unmarshal([]byte(testData), &req)

	enable := 1
	now := time.Now()
	silences := make([]*model.AlertSilence, 0)

	silences = append(silences, &model.AlertSilence{
		ID:          1,
		Cluster:     "chengde",
		Type:        1,
		Fingerprint: "28c89c4f51cb8e24",
		Status:      &enable,
		EndsAt:      now.Add(100 * time.Hour),
		StartsAt:    now.Add(-100 * time.Hour),
	})

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

	matcher1 := &model.Matcher{
		Name:  "alertname",
		Type:  "!=",
		Value: "DaemonSet滚动更新卡住",
	}
	matchers1 := make([]*model.Matcher, 0)
	matchers1 = append(matchers1, matcher1)

	matchersBy1, err := json.Marshal(&matchers1)
	if err != nil {
		t.Fatal(err)
	}

	silences = append(silences, &model.AlertSilence{
		ID:          3,
		Cluster:     "chengde",
		Type:        2,
		Fingerprint: "28c89c4f51cb8e24",
		Status:      &enable,
		EndsAt:      now.Add(100 * time.Hour),
		StartsAt:    now.Add(-100 * time.Hour),
		Matchers:    matchersBy1,
	})

	alertsServicer := v1.NewAlertsServicer(nil, nil)

	for _, v := range req.Alerts {
		silience, id := alertsServicer.IsSilenced(context.Background(), v, silences)

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
