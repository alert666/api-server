package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	SilenceDisabled = iota
	SilenceEnabled  = iota
	SilenceExpired  = iota
)

// AlertSilence 静默规则表
type AlertSilence struct {
	ID        int            `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time      `gorm:"column:created_at" json:"createdAt,omitempty"`
	UpdatedAt time.Time      `gorm:"column:updated_at" json:"updatedAt,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
	Cluster   string         `gorm:"column:cluster;type:varchar(128);not null;index:idx_cluster_status_ends_starts,priority:1;comment:所属集群/租户" json:"cluster"`
	Status    *int           `gorm:"column:status;type:tinyint;default:1;index:idx_cluster_status_ends_starts,priority:2;comment:状态 0:禁用 1: 启用 2:过期" json:"status"`
	EndsAt    time.Time      `gorm:"column:ends_at;not null;index:idx_cluster_status_ends_starts,priority:3;comment:结束时间" json:"endsAt"`
	StartsAt  time.Time      `gorm:"column:starts_at;not null;index:idx_cluster_status_ends_starts,priority:4;comment:开始时间" json:"startsAt"`
	Matchers  datatypes.JSON `gorm:"column:matchers;type:json;not null;comment:匹配器集合" json:"matchers"`
	CreatedBy string         `gorm:"column:created_by;type:varchar(64)" json:"createdBy"`
	Comment   string         `gorm:"column:comment;type:text;comment:静默原因" json:"comment"`
}

// Matcher 匹配器具体结构
type Matcher struct {
	Name  string `json:"name"`  // 标签名
	Value string `json:"value"` // 标签值
	Type  string `json:"type"`  // 操作符: =, !=, =~, !~
}
