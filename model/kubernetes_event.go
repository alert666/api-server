package model

import (
	"time"
)

// KubernetesEvent Kubernetes 事件模型
type KubernetesEvent struct {
	ID                 int64     `gorm:"column:id;primaryKey;autoIncrement;comment:自增主键" json:"id"`
	Cluster            string    `gorm:"column:cluster;type:varchar(128);not null;uniqueIndex:idx_kube_event_unique;index:idx_cluster_reason_time;index:idx_resource;index:idx_cluster_time;comment:所属集群名称" json:"cluster"`
	Namespace          string    `gorm:"column:namespace;type:varchar(64);not null;uniqueIndex:idx_kube_event_unique;index:idx_resource;comment:资源所属命名空间" json:"namespace"`
	MetadataName       string    `gorm:"column:metadata_name;type:varchar(256);not null;uniqueIndex:idx_kube_event_unique;comment:事件自身的元数据名称(唯一标识)" json:"metadataName"`
	Reason             string    `gorm:"column:reason;type:varchar(128);index:idx_cluster_reason_time;comment:事件原因" json:"reason"`
	Name               string    `gorm:"column:name;type:varchar(256);index:idx_resource;comment:关联资源名称" json:"name"`
	Kind               string    `gorm:"column:kind;type:varchar(64);index:idx_resource;comment:关联资源类型" json:"kind"`
	LastSeen           time.Time `gorm:"column:last_seen;type:datetime;index:idx_cluster_reason_time;index:idx_resource;index:idx_cluster_time;index:idx_last_seen;comment:最近一次发生时间" json:"lastSeen"`
	FirstSeen          time.Time `gorm:"column:first_seen;type:datetime;comment:第一次发生时间" json:"firstSeen"`
	Type               string    `gorm:"column:type;type:varchar(16);comment:事件类型(Normal/Warning)" json:"type"`
	Message            string    `gorm:"column:message;type:text;comment:事件详细消息描述" json:"message"`
	Count              int32     `gorm:"column:count;default:1;comment:事件重复发生次数" json:"count"`
	ReportingComponent string    `gorm:"column:reporting_component;type:varchar(128);comment:报告此事件的组件" json:"reportingComponent"`
	ReportingInstance  string    `gorm:"column:reporting_instance;type:varchar(128);comment:报告此事件的实例(通常是Node名)" json:"reportingInstance"`
	ApiVersion         string    `gorm:"column:api_version;type:varchar(64);comment:API版本" json:"apiVersion"`
}

// TableName 定义该模型对应的数据库表名
func (*KubernetesEvent) TableName() string {
	return "kubernetes_events"
}
