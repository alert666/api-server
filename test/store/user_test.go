package store_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/qinquanliuxiang666/alertmanager/base/types"
	"github.com/qinquanliuxiang666/alertmanager/model"
	"github.com/qinquanliuxiang666/alertmanager/pkg/alert"
	"github.com/qinquanliuxiang666/alertmanager/store"
)

func TestCreateTable(t *testing.T) {
	if err := db.AutoMigrate(&model.Oauth2User{}); err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
}

func TestStoreQ(t *testing.T) {
	err := store.Q.Transaction(func(tx *store.Query) error {
		tx.WithContext(context.Background()).AlertSendRecord.Create(&model.AlertSendRecord{
			ID:         2,
			SendStatus: "sss",
		})
		tx.WithContext(context.Background()).AlertSendRecord.Create(&model.AlertSendRecord{
			ID:         1,
			SendStatus: "sss",
		})
		return nil
	})

	if err != nil {
		t.Fatal(err)
	}
}

const (
	channel = `{"ID":1,"createdAt":"2026-03-23T22:26:23+08:00","updatedAt":"2026-03-23T22:26:24+08:00","Name":"feishu","Type":"feishuApp","Status":1,"AggregationStatus":1,"Config":{"app_id":"cli_a9348075d4f8dcc7","app_secret":"c9eSiKuHzfPp2G7f0SLNhbasSHuyIetp","receive_id":"oc_1b331aaade15126c200d94627d8aa2a0","receive_id_type":"chat_id"},"Description":"使用飞书应用发送告警","AlertTemplateID":1,"alert_template":{"ID":1,"createdAt":"2026-03-22T21:30:04+08:00","updatedAt":"2026-03-29T16:27:06.56+08:00","Name":"飞书告警模板","Description":"更新测试1","Template":"template_id: \"AAqK947a7l70i\"\ntemplate_version_name: \"1.0.8\"\ntemplate_variable:\n  alertName: {{ printf \"%q\" (index .Labels \"alertname\") }}\n  alertDescribe: {{ index .Annotations \"description\" | printf \"\u0026nbsp;\u0026nbsp;\u0026nbsp;\u0026nbsp;%s\" | printf \"%q\" }}\n  alertCluster: {{ printf \"%q\" (index .Labels \"cluster\") }}\n  alertLevel: {{ printf \"%q\" (index .Labels \"severity\") }}\n  alertStartTime: {{ printf \"%q\" (timeFormat .StartsAt) }}\n  alertEndTime: {{ getEndTime .EndsAt \"告警未恢复\"  }}\n  alertUser: \"\u003cat id=28c4bfgf\u003e\u003c/at\u003e\"\n  disableSelect: false\n","AggregationTemplate":"{{- if .Alerts -}}\n{{- $first := index .Alerts 0 -}}\n{{- $count := len .Alerts -}}\n{{- /* 先在开头计算好所有变量，不产生任何渲染输出 */ -}}\n{{- $fullDesc := \"\" -}}\n{{- range $i, $v := .Alerts -}}\n  {{- if lt $i 3 -}}\n    {{- $line := printf \"%d. %s\\n\" (add $i 1) (index $v.Annotations \"description\") -}}\n    {{- $fullDesc = printf \"%s%s\" $fullDesc $line -}}\n  {{- end -}}\n{{- end -}}\n{{- if gt $count 3 -}}\n  {{- $footer := printf \"---\\n💡 当前已聚合 %d 条告警，仅展示前 3 条。\" $count -}}\n  {{- $fullDesc = printf \"%s%s\" $fullDesc $footer -}}\n{{- end -}}\n{{- /* 下面才是真正的 YAML 结构输出 */ -}}\ntemplate_id: \"AAqK947a7l70i\"\ntemplate_version_name: \"1.0.8\"\ntemplate_variable:\n  alertName: {{ if gt $count 1 }}{{ printf \"[聚合%d条告警] %s\" $count (index $first.Labels \"alertname\") | printf \"%q\" }}{{ else }}{{ printf \"%q\" (index $first.Labels \"alertname\") }}{{ end }}\n  alertCluster: {{ printf \"%q\" (index $first.Labels \"cluster\") }}\n  alertLevel: {{ printf \"%q\" (index $first.Labels \"severity\") }}\n  alertStartTime: {{ printf \"%q\" (timeFormat $first.StartsAt) }}\n  alertEndTime: {{ if $first.EndsAt.IsZero }}{{ printf \"%q\" \"告警未恢复\" }}{{ else }}{{ printf \"%q\" (timeFormat $first.EndsAt) }}{{ end }}\n  alertUser: \"\u003cat id=28c4bfgf\u003e\u003c/at\u003e\"\n  disableSelect: false\n  alertDescribe: {{ printf \"%q\" $fullDesc }}\n{{- end -}}\n"}}
`

	alerts = `{"FiringErr":null,"ResolvedErr":null,"FiringAlerts":[{"status":"firing","labels":{"alertgroup":"ContainerMemoryAlerts","alertname":"ContainerMemoryUsageHigh","beta_kubernetes_io_arch":"amd64","beta_kubernetes_io_instance_type":"rke2","beta_kubernetes_io_os":"linux","cluster":"local","container":"cilium-agent","id":"/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod193c20d9_34af_4b69_b119_2e75aa9023d1.slice/cri-containerd-423f66c54536da89e0a167ccd93719299bcebf99a7e948f6fa02640116f90f04.scope","image":"docker.io/rancher/mirrored-cilium-cilium:v1.18.1","instance":"node0","job":"cadvisor","kubernetes_io_arch":"amd64","kubernetes_io_hostname":"node0","kubernetes_io_os":"linux","metrics_path":"/metrics/cadvisor","name":"423f66c54536da89e0a167ccd93719299bcebf99a7e948f6fa02640116f90f04","namespace":"kube-system","node":"node0","node_kubernetes_io_instance_type":"rke2","node_role_kubernetes_io_control_plane":"true","node_role_kubernetes_io_etcd":"true","node_role_kubernetes_io_master":"true","pod":"cilium-95zkx","severity":"critical","team":"infrastructure","vmagent_ha":"monitoring/vm-agent"},"annotations":{"description":"Pod cilium-95zkx (命名空间: kube-system) 的容器内存使用量已超过 300MB (当前值: 465.72 MB)","summary":"容器内存使用过高 (cilium-95zkx)"},"startsAt":"2026-03-29T09:48:10Z","endsAt":"0001-01-01T00:00:00Z","generatorURL":"http://vmalert-vm-alert-778d57f7dd-lk89b:8080/vmalert/alert?group_id=5943003096686295237\u0026alert_id=16824412172183142391","fingerprint":"155d534f493b04b7"},{"status":"firing","labels":{"alertgroup":"ContainerMemoryAlerts","alertname":"ContainerMemoryUsageHigh","beta_kubernetes_io_arch":"amd64","beta_kubernetes_io_instance_type":"rke2","beta_kubernetes_io_os":"linux","cluster":"local","container":"kube-apiserver","id":"/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod021ef3e1fb15e22f61d582ef276f3303.slice/cri-containerd-c67122e22cbb94b59df95c7d732e5e1b7023785c46867e859fc3f5420fe0450d.scope","image":"docker.io/rancher/hardened-kubernetes:v1.33.5-rke2r1-build20250910","instance":"node0","job":"cadvisor","kubernetes_io_arch":"amd64","kubernetes_io_hostname":"node0","kubernetes_io_os":"linux","metrics_path":"/metrics/cadvisor","name":"c67122e22cbb94b59df95c7d732e5e1b7023785c46867e859fc3f5420fe0450d","namespace":"kube-system","node":"node0","node_kubernetes_io_instance_type":"rke2","node_role_kubernetes_io_control_plane":"true","node_role_kubernetes_io_etcd":"true","node_role_kubernetes_io_master":"true","pod":"kube-apiserver-node0","severity":"critical","team":"infrastructure","vmagent_ha":"monitoring/vm-agent"},"annotations":{"description":"Pod kube-apiserver-node0 (命名空间: kube-system) 的容器内存使用量已超过 300MB (当前值: 452.94 MB)","summary":"容器内存使用过高 (kube-apiserver-node0)"},"startsAt":"2026-03-29T09:48:10Z","endsAt":"0001-01-01T00:00:00Z","generatorURL":"http://vmalert-vm-alert-778d57f7dd-lk89b:8080/vmalert/alert?group_id=5943003096686295237\u0026alert_id=8277272836017799481","fingerprint":"64cca14012f123b1"},{"status":"firing","labels":{"alertgroup":"ContainerMemoryAlerts","alertname":"ContainerMemoryUsageHigh","beta_kubernetes_io_arch":"amd64","beta_kubernetes_io_instance_type":"rke2","beta_kubernetes_io_os":"linux","cluster":"local","container":"cilium-agent","id":"/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod543abf76_51b4_4bc1_bef7_3d7267535443.slice/cri-containerd-b3efc3e00b5240f0b986816059db7ad8b28621d614f255e2feb6705bfe497406.scope","image":"docker.io/rancher/mirrored-cilium-cilium:v1.18.1","instance":"node1","job":"cadvisor","kubernetes_io_arch":"amd64","kubernetes_io_hostname":"node1","kubernetes_io_os":"linux","metrics_path":"/metrics/cadvisor","name":"b3efc3e00b5240f0b986816059db7ad8b28621d614f255e2feb6705bfe497406","namespace":"kube-system","node":"node1","node_kubernetes_io_instance_type":"rke2","pod":"cilium-gbxb9","severity":"critical","team":"infrastructure","vmagent_ha":"monitoring/vm-agent"},"annotations":{"description":"Pod cilium-gbxb9 (命名空间: kube-system) 的容器内存使用量已超过 300MB (当前值: 472.28 MB)","summary":"容器内存使用过高 (cilium-gbxb9)"},"startsAt":"2026-03-29T09:48:10Z","endsAt":"0001-01-01T00:00:00Z","generatorURL":"http://vmalert-vm-alert-778d57f7dd-lk89b:8080/vmalert/alert?group_id=5943003096686295237\u0026alert_id=16450287045786006782","fingerprint":"57bacb6022b4386b"}],"ResolvedAlerts":[]}
`
)

func TestAL(t *testing.T) {
	var c *model.AlertChannel
	var cc *types.HandleAggregationSendResult
	if err := json.Unmarshal([]byte(channel), &c); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(alerts), &cc); err != nil {
		t.Fatal(err)
	}

	receiver := alert.NewAlertUtiler()
	receiver.SaveAggregationAlert(context.Background(), c, cc)

	time.Sleep(1 * time.Hour)
}
