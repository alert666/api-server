package model

import (
	"time"

	"gorm.io/gorm"
)

// AlertTemplate 告警模板表
type AlertTemplate struct {
	ID                  int            `gorm:"primaryKey" json:"id"`
	CreatedAt           time.Time      `gorm:"column:created_at" json:"createdAt,omitempty"`
	UpdatedAt           time.Time      `gorm:"column:updated_at" json:"updatedAt,omitempty"`
	DeletedAt           gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
	Name                string         `gorm:"column:name;type:varchar(100);not null;uniqueIndex;comment:模板名称" json:"name"`
	Description         string         `gorm:"column:description;type:varchar(255)" json:"description"`
	Template            string         `gorm:"column:template;type:text;not null;comment:单个告警(Markdown/HTML)模板" json:"template"`
	AggregationTemplate string         `gorm:"column:aggregation_template;type:text;comment:聚合告警(Markdown/HTML)模板" json:"aggregationTemplate"`
}
