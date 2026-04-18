package model

import "regexp"

// apiVersion: v1
// stringData:
//   alertmanager.yaml: |-
//     global:
//       resolve_timeout: 5m
//     inhibit_rules:
//     - source_matchers: [alertname="节点磁盘空间不足", severity="critical"]
//       target_matchers: [alertname="节点磁盘空间不足", severity="warning"]
//       equal: ['instance', 'device', 'mountpoint']
//     - source_matchers: [alertname="网络接口波动"]
//       target_matchers: [alertname="主机网络接口宕机"]
//       equal: ['instance', 'device']
//     receivers:
//     - name: 'prometheusalert'
//       webhook_configs:
//       - url: 'http://prometheus-alert-center.monitoring.svc.cluster.local:8080/prometheusalert?type=fs&tpl=prometheus-fsv2&fsurl=https://open.feishu.cn/open-apis/bot/v2/hook/57bbe881-c46f-4393-8827-21a177bb5911&at=ywzb@suanleme.cn'
//       - url: 'https://qqlx.net/api/v1/alerts?channelName=feishu'
//     route:
//       receiver: 'prometheusalert'
//       group_by: ['alertname', 'namespace', 'instance']
//       group_wait: 30s
//       group_interval: 5m
//       repeat_interval: 1h
//       routes:
//         - matchers:
//             - severity = "critical"
//           repeat_interval: 10m
// kind: Secret
// metadata:
//   creationTimestamp: "2025-11-04T09:02:32Z"
//   labels:
//     app.kubernetes.io/component: alert-router
//     app.kubernetes.io/instance: main
//     app.kubernetes.io/name: alertmanager
//     app.kubernetes.io/part-of: kube-prometheus
//     app.kubernetes.io/version: 0.27.0
//   name: alertmanager-main
//   namespace: monitoring
// type: Opaque

type AlertInhibitionRule struct {
	ID             int        `gorm:"primaryKey;autoIncrement" json:"id"`
	Name           string     `gorm:"type:varchar(64);comment:规则名称" json:"name"`
	SourceMatchers []*Matcher `gorm:"type:json;comment:源告警(抑制者)匹配器" json:"sourceMatchers"`
	TargetMatchers []*Matcher `gorm:"type:json;comment:目标告警(被抑制者)匹配器" json:"targetMatchers"`
	EqualLabels    []string   `gorm:"type:json;comment:必须相等的标签列表" json:"equalLabels"`
	Status         int        `gorm:"type:tinyint;default:1;comment:1启用 0禁用" json:"status"`
}

func (m *Matcher) Matches(labelValue string) bool {
	switch m.Type {
	case "=":
		return labelValue == m.Value
	case "!=":
		return labelValue != m.Value
	case "=~":
		match, _ := regexp.MatchString("^("+m.Value+")$", labelValue)
		return match
	case "!~":
		match, _ := regexp.MatchString("^("+m.Value+")$", labelValue)
		return !match
	}
	return false
}
