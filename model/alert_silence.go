package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// AlertSilence 静默规则表
type AlertSilence struct {
	ID        int            `gorm:"primaryKey;autoIncrement"`
	CreatedAt time.Time      `gorm:"column:created_at" json:"createdAt,omitempty"`
	UpdatedAt time.Time      `gorm:"column:updated_at" json:"updatedAt,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
	Cluster   string         `gorm:"type:varchar(128);index;comment:所属集群/租户"`
	Matchers  datatypes.JSON `gorm:"type:json;not null;comment:匹配器集合 [{name:x, value:y, type:z}]"`
	StartsAt  time.Time      `gorm:"index;comment:开始时间"`
	EndsAt    time.Time      `gorm:"index;comment:结束时间"`
	CreatedBy string         `gorm:"type:varchar(64)"`
	Comment   string         `gorm:"type:text;comment:静默原因"`
	Status    int            `gorm:"type:tinyint;default:1;comment:状态 1:启用 2:手动禁用"`
}

// Matcher 匹配器具体结构
type Matcher struct {
	Name  string `json:"name"`  // 标签名
	Value string `json:"value"` // 标签值
	Type  string `json:"type"`  // 操作符: =, !=, =~, !~
}
