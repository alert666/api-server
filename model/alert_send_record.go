package model

import (
	"fmt"
	"time"
)

const (
	AlertSendRecordStatusSuccess = "success"
	AlertSendRecordStatusFailed  = "failed"
)

// AlertSendRecord 告警发送明细/日志表
type AlertSendRecord struct {
	ID                int             `gorm:"primaryKey;autoIncrement" json:"id"`
	CreatedAt         time.Time       `gorm:"column:created_at" json:"createdAt,omitempty"`
	UpdatedAt         time.Time       `gorm:"column:updated_at" json:"updatedAt,omitempty"`
	AlertHistory      []*AlertHistory `gorm:"foreignKey:AlertSendRecordID" json:"alertHistory"`
	SendStatus        string          `gorm:"column:send_status;type:varchar(32);not null;index;comment:发送状态(success, failed)" json:"sendStatus"`
	ErrorMessage      string          `gorm:"column:error_message;type:text;comment:如果发送失败，记录失败的报错详情(供排查)" json:"errorMessage"`
	ExternalMessageID string          `gorm:"column:external_message_id;type:varchar(255);index;comment:第三方平台返回的消息ID(如飞书的 message_id)" json:"externalMessageID"`
}

func (*AlertSendRecord) TableName() string {
	return "alert_send_records"
}

func UpdateSendRecordStatus(sendErr error) *AlertSendRecord {
	record := &AlertSendRecord{}
	if sendErr == nil {
		record.SendStatus = AlertSendRecordStatusSuccess
	} else {
		record.SendStatus = AlertSendRecordStatusFailed
		record.ErrorMessage = fmt.Sprintf("record.ErrorMessage \n%s", sendErr.Error())
	}
	return record
}
