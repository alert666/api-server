package types

import (
	"encoding/json"
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
	AlertChannel    *model.AlertChannel
	NotifyAlerts    *NotifyAlerts
	AlertReceiveReq *AlertReceiveReq
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
