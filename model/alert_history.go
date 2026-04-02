package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// AlertHistory 告警历史记录表
type AlertHistory struct {
	ID                int              `gorm:"column:id;primaryKey;autoIncrement;comment:主键ID" json:"id"`
	Cluster           string           `gorm:"column:cluster;type:varchar(128);index;comment:租户" json:"cluster"`
	CreatedAt         time.Time        `gorm:"column:created_at;type:datetime;autoCreateTime;comment:本条记录存入数据库的时间" json:"createdAt"`
	Fingerprint       string           `gorm:"column:fingerprint;type:varchar(128);not null;uniqueIndex:idx_alert_unique;comment:指纹" json:"fingerprint"`
	StartsAt          time.Time        `gorm:"column:starts_at;type:datetime(3);precision:3;not null;uniqueIndex:idx_alert_unique;comment:开始时间" json:"startsAt"`
	EndsAt            *time.Time       `gorm:"column:ends_at;type:datetime;index:idx_ends_at;comment:告警恢复时间(未恢复则为NULL)" json:"endsAt"`
	Status            string           `gorm:"column:status;type:varchar(32);not null;index;comment:告警状态(如: firing, resolved)" json:"status"`
	Alertname         string           `gorm:"column:alertname;type:varchar(255);not null;index;comment:告警名称" json:"alertname"`
	Severity          string           `gorm:"column:severity;type:varchar(32);index;comment:告警级别(如: critical, warning, info)" json:"severity"`
	Instance          string           `gorm:"column:instance;type:varchar(255);index;comment:告警发生的实例(如IP或主机名)" json:"instance"`
	Labels            datatypes.JSON   `gorm:"column:labels;type:json;comment:告警标签集合" json:"labels"`
	Annotations       datatypes.JSON   `gorm:"column:annotations;type:json;comment:告警详情/注解" json:"annotations"`
	AlertChannelID    int              `gorm:"column:alert_channel_id;not null;index;comment:关联的告警发送通道ID" json:"alertChannelId"`
	AlertChannel      *AlertChannel    `gorm:"foreignKey:AlertChannelID" json:"alertChannel"`
	AlertSendRecordID int              `gorm:"column:alert_send_record_id;not null;index;comment:关联的告警发送记录ID" json:"alertSendRecordID"`
	AlertSendRecord   *AlertSendRecord `gorm:"foreignKey:AlertSendRecordID" json:"alertSendRecord"`
	SendCount         int              `gorm:"column:send_count;type:int;size:3;comment:告警发送次数" json:"sendCount"`
}

func (*AlertHistory) TableName() string {
	return "alert_historys"
}

// BeforeSave GORM 钩子：在保存到数据库前，统一截断时间精度到毫秒
// 这样可以保证：写入数据库的值 == 内存中的值 == 未来查询的值
func (a *AlertHistory) BeforeSave(tx *gorm.DB) (err error) {
	a.StartsAt = a.StartsAt.Truncate(time.Millisecond)
	if a.EndsAt != nil {
		t := a.EndsAt.Truncate(time.Millisecond)
		a.EndsAt = &t
	}
	return
}
