package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// AlertHistory 告警历史记录表
type AlertHistory struct {
	ID                int              `gorm:"column:id;primaryKey;autoIncrement;comment:主键ID" json:"id"`
	CreatedAt         time.Time        `gorm:"column:created_at;type:datetime;autoCreateTime;comment:本条记录存入数据库的时间" json:"createdAt"`
	Fingerprint       string           `gorm:"column:fingerprint;type:varchar(128);not null;uniqueIndex:uk_alert_identity,priority:1;comment:指纹" json:"fingerprint"`
	StartsAt          time.Time        `gorm:"column:starts_at;type:datetime(3);precision:3;not null;uniqueIndex:uk_alert_identity,priority:2;comment:开始时间" json:"startsAt"`
	Cluster           string           `gorm:"column:cluster;type:varchar(128);not null;default:'default';uniqueIndex:uk_alert_identity,priority:3;index:idx_status_cluster,priority:2;comment:租户" json:"cluster"`
	Status            string           `gorm:"column:status;type:varchar(32);not null;index:idx_status_cluster,priority:1;comment:告警状态" json:"status"`
	EndsAt            *time.Time       `gorm:"column:ends_at;type:datetime;index:idx_ends_at;comment:告警恢复时间" json:"endsAt"`
	AlertChannelID    int              `gorm:"column:alert_channel_id;not null;index:idx_channel_id;comment:关联通道ID" json:"alertChannelId"`
	AlertSendRecordID *int             `gorm:"column:alert_send_record_id;index:idx_send_record_id;comment:关联发送记录ID和分组ID" json:"alertSendRecordID"`
	AlertSilenceID    int              `gorm:"column:alert_silence_id;index:idx_history_silence_id;comment:关联静默规则ID" json:"alertSilenceID"`
	Alertname         string           `gorm:"column:alertname;type:varchar(255);not null" json:"alertname"`
	Severity          string           `gorm:"column:severity;type:varchar(32)" json:"severity"`
	Instance          string           `gorm:"column:instance;type:varchar(255)" json:"instance"`
	Labels            datatypes.JSON   `gorm:"column:labels;type:json" json:"labels"`
	Annotations       datatypes.JSON   `gorm:"column:annotations;type:json" json:"annotations"`
	SendCount         int              `gorm:"column:send_count;type:int;size:3" json:"sendCount"`
	IsSilenced        bool             `gorm:"column:is_silenced;default:false" json:"isSilenced"`
	AlertChannel      *AlertChannel    `gorm:"foreignKey:AlertChannelID" json:"alertChannel"`
	AlertSendRecord   *AlertSendRecord `gorm:"foreignKey:AlertSendRecordID" json:"alertSendRecord"`
	AlertSilence      *AlertSilence    `gorm:"foreignKey:AlertSilenceID" json:"alertSilence"`
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
