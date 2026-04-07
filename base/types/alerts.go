package types

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/qinquanliuxiang666/alertmanager/model"
	"gorm.io/datatypes"
)

// AlertReceiveReq 是 Alertmanager 发送的 Webhook 顶层 JSON 结构
type AlertReceiveReq struct {
	ChannelName       string            `form:"channelName" binding:"required"`
	Receiver          string            `json:"receiver"`
	Status            string            `json:"status"` // "firing" or "resolved"
	Alerts            []*Alert          `json:"alerts"`
	GroupLabels       map[string]string `json:"groupLabels"`
	CommonLabels      map[string]string `json:"commonLabels"`
	CommonAnnotations map[string]string `json:"commonAnnotations"`
	ExternalURL       string            `json:"externalURL"`
	Version           string            `json:"version"`
	GroupKey          string            `json:"groupKey"`
	TruncatedAlerts   uint64            `json:"truncatedAlerts"`
}

// Alert 代表单条告警的详情
type Alert struct {
	Status       string            `json:"status"` // "firing" or "resolved"
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       *time.Time        `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
	Fingerprint  string            `json:"fingerprint"`
	IsSilenced   bool              `json:"isSilenced"`
	SilenceID    int               `json:"silenceID"`
}

// NewTestAlertReceiveReq 测试模板使用的告警数据
func NewTestAlertReceiveReq() *AlertReceiveReq {
	now := time.Now()
	promHost := "http://prometheus.monitoring.svc:9090"
	promQL := "(node_filesystem_avail_bytes%7Bmountpoint%3D%22%2F%22%7D+%2F+node_filesystem_size_bytes%7Bmountpoint%3D%22%2F%22%7D%29+*+100+%3C+10"

	return &AlertReceiveReq{
		ChannelName: "feishu",
		Receiver:    "feishu-receiver",
		Status:      "firing",
		Version:     "4",
		ExternalURL: "http://alertmanager.qqlx.net",
		GroupKey:    "{}/{}:{alertname=\"NodeDiskUsageHigh\", cluster=\"local\"}",
		GroupLabels: map[string]string{
			"alertname": "NodeDiskUsageHigh",
			"cluster":   "local",
		},
		CommonLabels: map[string]string{
			"alertgroup": "HostDiskAlerts",
			"alertname":  "NodeDiskUsageHigh",
			"cluster":    "local",
			"severity":   "critical",
			"team":       "infrastructure",
		},
		CommonAnnotations: map[string]string{},
		TruncatedAlerts:   0,
		Alerts: []*Alert{
			{
				Status:      "firing",
				Fingerprint: "20035b789c29547a",
				StartsAt:    now.Add(-10 * time.Minute), // 10分钟前开始
				EndsAt:      nil,                        // 正在告警，设为 nil
				// 标准 Prometheus 格式：/graph?g0.expr=...&g0.tab=1
				GeneratorURL: fmt.Sprintf("%s/graph?g0.expr=%s&g0.tab=1", promHost, promQL),
				Labels: map[string]string{
					"alertname":  "NodeDiskUsageHigh",
					"cluster":    "local",
					"instance":   "10.0.0.10:9100",
					"severity":   "critical",
					"device":     "/dev/sda2",
					"mountpoint": "/",
				},
				Annotations: map[string]string{
					"summary":     "节点磁盘使用率过高 (10.0.0.10:9100)",
					"description": "节点 10.0.0.10 的根分区使用率已超过 90% (当前值: 91.42%)",
				},
			},
			{
				Status:       "firing",
				Fingerprint:  "87044fca2101f4c3",
				StartsAt:     now.Add(-5 * time.Minute), // 5分钟前开始
				EndsAt:       nil,
				GeneratorURL: fmt.Sprintf("%s/graph?g0.expr=%s&g0.tab=1", promHost, promQL),
				Labels: map[string]string{
					"alertname":  "NodeDiskUsageHigh",
					"cluster":    "local",
					"instance":   "10.0.0.11:9100",
					"severity":   "critical",
					"device":     "/dev/sda2",
					"mountpoint": "/",
				},
				Annotations: map[string]string{
					"summary":     "节点磁盘使用率过高 (10.0.0.11:9100)",
					"description": "节点 10.0.0.11 的根分区使用率已超过 90% (当前值: 92.60%)",
				},
			},
		},
	}
}

// 辅助函数：将业务 Alert 转换为 DB Model
func ConvertToModel(tenantKey string, a *Alert, channelID int) (*model.AlertHistory, error) {
	labelByte, err := json.Marshal(a.Labels)
	if err != nil {
		return nil, err
	}
	annotationsByte, err := json.Marshal(a.Annotations)
	if err != nil {
		return nil, err
	}
	return &model.AlertHistory{
		Fingerprint:    a.Fingerprint,
		StartsAt:       a.StartsAt,
		Cluster:        a.Labels[tenantKey], // 获取告警数据里 tenantKey 对应的值, 用于存储到数据库中区分租户
		EndsAt:         a.EndsAt,
		Status:         a.Status,
		Alertname:      a.Labels["alertname"],
		Severity:       a.Labels["severity"],
		Instance:       a.Labels["instance"],
		Labels:         datatypes.JSON(labelByte),
		Annotations:    datatypes.JSON(annotationsByte),
		AlertChannelID: channelID,
		SendCount:      1,
	}, nil
}

// DeepCopy 创建 AlertReceiveReq 的深拷贝，确保在处理过程中数据不被修改
func (receiver *AlertReceiveReq) DeepCopy() *AlertReceiveReq {
	return &AlertReceiveReq{
		ChannelName:       receiver.ChannelName,
		Receiver:          receiver.Receiver,
		Status:            receiver.Status,
		GroupLabels:       receiver.GroupLabels,
		CommonLabels:      receiver.CommonLabels,
		CommonAnnotations: receiver.CommonAnnotations,
		ExternalURL:       receiver.ExternalURL,
		Version:           receiver.Version,
		GroupKey:          receiver.GroupKey,
		TruncatedAlerts:   receiver.TruncatedAlerts,
	}
}

// NotifyReq 是内部服务处理告警发送的请求结构，包含了告警发送通道信息、告警详情和原始接收请求
type NotifyReq struct {
	AlertChannel     *model.AlertChannel
	AlertReceiveReq  *AlertReceiveReq
	TenantValue      string
	AlertMap         *AlertMap
	ExistingAlertMap map[string]*model.AlertHistory
	AlertArry        *AlertArry
}

func NewNotifyReq() *NotifyReq {
	return &NotifyReq{
		AlertArry: &AlertArry{},
		AlertMap:  &AlertMap{},
	}
}

type AlertArry struct {
	FiringAlertArry   []*Alert
	ResolvedAlertArry []*Alert
}

type AlertMap struct {
	FiringAlertMap   map[string]*Alert
	ResolvedAlertMap map[string]*Alert
	SilencedAlertMap map[string]*Alert
}

// NotifyAlerts 代表一次告警发送中所有的告警详情，分为正在触发的告警和已恢复的告警两类
type NotifyAlerts struct {
	FiringAlerts   []*Alert
	ResolvedAlerts []*Alert
}

// NotifySendResult 代表一次告警发送的结果，包含聚合发送结果和单条发送结果
type NotifySendResult struct {
	AggregationSendResult *AggregationSendResult
	SingleSendResult      []*SingleSendResult
}

// AggregationSendResult 代表批量发送的结果，包含发送错误信息和对应的告警列表
type AggregationSendResult struct {
	FiringErr   error
	ResolvedErr error
}

// SingleSendResult 代表单条告警发送的结果，包含告警详情和发送错误信息
type SingleSendResult struct {
	Alert   *Alert
	SendErr error
}
