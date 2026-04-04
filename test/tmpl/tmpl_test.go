package tmpl_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
	"text/template"
	"time"

	"github.com/qinquanliuxiang666/alertmanager/base/types"
	"sigs.k8s.io/yaml"
)

var tplStr string = `template_id: "AAqK947a7l70i"
template_version_name: "1.0.4"
template_variable:
  alertName: {{ printf "%q" (index .Labels "alertname") }}
  alertDescribe: {{ index .Annotations "description" | printf "&nbsp;&nbsp;&nbsp;&nbsp;%s" | printf "%q" }}
  alertCluster: {{ printf "%q" (index .Labels "cluster") }}
  alertLevel: {{ printf "%q" (index .Labels "severity") }}
  alertStartTime: {{ printf "%q" (timeFormat .StartsAt) }}
  alertUser: "<at id=28c4bfgf></at>"
  disableSelect: false`

var alertStr = `{"alerts":[{"annotations":{"description":"节点  的根分区使用率已超过 10% (当前值: 11.42%)","summary":"节点磁盘使用率过高 (10.0.0.10:9100)"},"endsAt":"0001-01-01T00:00:00Z","fingerprint":"20035b789c29547a","generatorURL":"http://vmalert-vm-alert-778d57f7dd-lk89b:8080/vmalert/alert?group_id=797613645340499355\u0026alert_id=1890247322429592058","labels":{"alertgroup":"HostDiskAlerts","alertname":"NodeDiskUsageHigh","cluster":"local","container":"node-exporter","device":"/dev/sda2","endpoint":"metrics","fstype":"ext4","instance":"10.0.0.10:9100","job":"node-exporter","mountpoint":"/","namespace":"monitoring","pod":"node-exporter-pdvgq","service":"node-exporter","severity":"critical","team":"infrastructure","vmagent_ha":"monitoring/vm-agent"},"startsAt":"2026-03-16T16:03:00Z","status":"firing"},{"annotations":{"description":"节点  的根分区使用率已超过 10% (当前值: 11.60%)","summary":"节点磁盘使用率过高 (10.0.0.11:9100)"},"endsAt":"0001-01-01T00:00:00Z","fingerprint":"87044fca2101f4c3","generatorURL":"http://vmalert-vm-alert-778d57f7dd-lk89b:8080/vmalert/alert?group_id=797613645340499355\u0026alert_id=2173403069717552903","labels":{"alertgroup":"HostDiskAlerts","alertname":"NodeDiskUsageHigh","cluster":"local","container":"node-exporter","device":"/dev/sda2","endpoint":"metrics","fstype":"ext4","instance":"10.0.0.11:9100","job":"node-exporter","mountpoint":"/","namespace":"monitoring","pod":"node-exporter-x7dpk","service":"node-exporter","severity":"critical","team":"infrastructure","vmagent_ha":"monitoring/vm-agent"},"startsAt":"2026-03-16T16:03:00Z","status":"firing"}],"commonAnnotations":{},"commonLabels":{"alertgroup":"HostDiskAlerts","alertname":"NodeDiskUsageHigh","cluster":"local","container":"node-exporter","device":"/dev/sda2","endpoint":"metrics","fstype":"ext4","job":"node-exporter","mountpoint":"/","namespace":"monitoring","service":"node-exporter","severity":"critical","team":"infrastructure","vmagent_ha":"monitoring/vm-agent"},"externalURL":"http://alertmanager.yourdomain.com","groupKey":"{}/{}:{alertname=\"NodeDiskUsageHigh\", cluster=\"local\"}","groupLabels":{"alertname":"NodeDiskUsageHigh","cluster":"local"},"receiver":"feishu-receiver","status":"firing","truncatedAlerts":0,"version":"4"}`

// 注册自定义函数，方便在模板里格式化时间
var funcMap = template.FuncMap{
	"timeFormat": func(t time.Time) string {
		return t.Local().Format("2006-01-02 15:04:05")
	},
	"add": func(a, b int) int {
		return a + b
	},
}

func TestTmpl(t *testing.T) {
	alert := new(types.AlertReceiveReq)
	if err := json.Unmarshal([]byte(alertStr), alert); err != nil {
		t.Fatal(err)
	}

	tmpl, err := template.New("alert").Funcs(funcMap).Parse(tplStr)
	if err != nil {
		t.Fatal(err)
	}

	for _, v := range alert.Alerts {
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, v); err != nil {
			t.Fatal(err)
		}
		jsonBytes, err := yaml.YAMLToJSON(buf.Bytes())
		if err != nil {
			t.Fatalf("yaml to json error: %s", err)
		}
		ssss, err := json.Marshal(string(jsonBytes))
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println("☀️------------------------------------☀️")
		fmt.Println(string(ssss))
		fmt.Println("🌙------------------------------------🌙")
	}
}

var aggregationTpl = `{{- if .Alerts -}}
{{- $first := index .Alerts 0 -}}
{{- $count := len .Alerts -}}
template_id: "AAqK947a7l70i"
template_version_name: "1.0.4"
template_variable:
  alertName: {{ if gt $count 1 }}{{ printf "[聚合%d条告警] %s" $count (index $first.Labels "alertname") | printf "%q" }}{{ else }}{{ printf "%q" (index $first.Labels "alertname") }}{{ end }}

  alertCluster: {{ printf "%q" (index $first.Labels "cluster") }}
  alertLevel: {{ printf "%q" (index $first.Labels "severity") }}
  alertStartTime: {{ printf "%q" (timeFormat $first.StartsAt) }}
  alertUser: "<at id=28c4bfgf></at>"
  disableSelect: false

  # 2. 描述内容优化
  alertDescribe: |
    {{- range $i, $v := .Alerts -}}
    {{- if lt $i 3 }}
    {{ add $i 1 }}. {{ index $v.Annotations "description" -}}
    {{- end -}}
    {{- end }}
    {{- if gt $count 3 }}
    ---
    💡 当前已聚合 {{ $count }} 条告警，仅展示前 3 条。完整信息请点击下方详情查看。
    {{- end }}
{{- end -}}`

func TestAggregationAlert(t *testing.T) {
	alert := new(types.AlertReceiveReq)
	if err := json.Unmarshal([]byte(alertStr), alert); err != nil {
		t.Fatal(err)
	}

	tmpl, err := template.New("alert").Funcs(funcMap).Parse(aggregationTpl)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, alert); err != nil {
		t.Fatal(err)
	}
	jsonBytes, err := yaml.YAMLToJSON(buf.Bytes())
	if err != nil {
		t.Fatalf("yaml to json error: %s", err)
	}
	fmt.Println("☀️------------------------------------☀️")
	fmt.Println(string(jsonBytes))
	fmt.Println("🌙------------------------------------🌙")

	content, err := json.Marshal(string(jsonBytes))
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("☀️------------------------------------☀️")
	fmt.Println(string(content))
	fmt.Println("🌙------------------------------------🌙")
}

func TestData(t *testing.T) {
	alert := new(types.AlertReceiveReq)
	if err := json.Unmarshal([]byte(alertStr), alert); err != nil {
		t.Fatal(err)
	}

	fmt.Println("☀️------------------------------------☀️")
	fmt.Println(alert.Alerts[0].EndsAt.Local().Format("2006-01-02 15:04:05"))
	fmt.Println("🌙------------------------------------🌙")

	fmt.Println("☀️------------------------------------☀️")
	fmt.Println(alert.Alerts[0].EndsAt.IsZero())
	fmt.Println("🌙------------------------------------🌙")
}
