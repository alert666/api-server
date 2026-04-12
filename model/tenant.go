package model

import "time"

type Tenant struct {
	ID          int64     `gorm:"column:id;primarykey;autoIncrement" json:"id"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updatedAt"`
	Name        string    `gorm:"column:name" json:"name"`
	Description string    `gorm:"column:description" json:"description"`
}

func (receiver *Tenant) TableName() string {
	return "tenants"
}
