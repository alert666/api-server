package model

import (
	"time"

	"gorm.io/gorm"
)

type IDCHeartbeat struct {
	ID                 int            `gorm:"column:id;primaryKey;autoIncrement;comment:自增主键" json:"id"`
	CreatedAt          time.Time      `gorm:"column:created_at" json:"createdAt,omitempty"`
	UpdatedAt          time.Time      `gorm:"column:updated_at" json:"updatedAt,omitempty"`
	DeletedAt          gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
	Cluster            string         `gorm:"column:cluster;type:varchar(128);not null;index:idx_cluster_node,priority:1;comment:集群名称" json:"cluster"`
	Node               string         `gorm:"column:node;type:varchar(128);not null;index:idx_cluster_node,priority:2;comment:节点名称" json:"node"`
	IP                 string         `gorm:"column:ip;type:varchar(64);not null;index;comment:节点IP地址" json:"ip"`
	HeartbeatTimestamp int64          `gorm:"column:heartbeat_timestamp;type:bigint;not null;index;comment:心跳时间戳(毫秒)" json:"heartbeatTimestamp"`
}

// TableName 定义该模型对应的数据库表名
func (*IDCHeartbeat) TableName() string {
	return "idc_heartbeats"
}
