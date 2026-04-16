package model

import (
	"encoding/json"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ChannelType 定义告警渠道的类型
type ChannelType string
type ChannelStatus int
type AggregationStatus int

const (
	ChannelTypeFeishuApp  ChannelType = "feishuApp"
	ChannelTypeFeishuBoot ChannelType = "feishuBoot"
	ChannelTypeWebhook    ChannelType = "webhook"
)

// ChannelStatus 定义告警渠道的状态

const (
	StatusDisabled      int = 0 // 停用
	StatusEnabled       int = 1 // 启用
	AggregationDisabled int = 0 // 启用
	AggregationEnabled  int = 1 // 启用
)

// AlertChannel 告警渠道表
type AlertChannel struct {
	ID                int            `gorm:"primarykey;comment:主键ID" json:"id"`
	CreatedAt         time.Time      `gorm:"column:created_at" json:"createdAt,omitempty"`
	UpdatedAt         time.Time      `gorm:"column:updated_at" json:"updatedAt,omitempty"`
	DeletedAt         gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
	Name              string         `gorm:"column:name;type:varchar(100);not null;uniqueIndex;comment:告警渠道名称(如: SRE团队钉钉群)" json:"name"`
	Type              ChannelType    `gorm:"column:type;type:varchar(50);not null;index;comment:渠道类型(feishuApp/feishuBoot/webhook)" json:"type"`
	Status            *int           `gorm:"column:status;type:tinyint;not null;default:1;index;comment:状态(0-停用, 1-启用)" json:"status"`
	AggregationStatus *int           `gorm:"column:aggregation_status;type:tinyint;not null;index;comment:状态(0-停用, 1-启用)" json:"aggregationStatus"`
	Config            datatypes.JSON `gorm:"column:config;type:json;not null;comment:渠道动态配置(JSON格式)" json:"config"`
	Description       string         `gorm:"column:description;type:varchar(255);comment:描述与备注" json:"description"`
	AlertTemplateID   int            `gorm:"column:alert_template_id;index;comment:绑定的告警模板ID" json:"alertTemplateID"`
	AlertTemplate     *AlertTemplate `gorm:"foreignKey:AlertTemplateID" json:"alertTemplate,omitempty"`
}

// TableName 指定表名
func (*AlertChannel) TableName() string {
	return "alert_channels"
}

// FeishuAppConfig 飞书自建应用配置
type FeishuAppConfig struct {
	AppID         string `json:"app_id"`          // 飞书应用的 App ID
	AppSecret     string `json:"app_secret"`      // 飞书应用的 App Secret
	ReceiveIdType string `json:"receive_id_type"` // 接收者类型: open_id, user_id, email, chat_id
	ReceiveId     string `json:"receive_id"`      // 接收者ID (具体是哪个用户或哪个群)
}

// WebhookConfig 通用 Webhook 配置
type WebhookConfig struct {
	URL     string            `json:"url"`
	Secret  string            `json:"secret,omitempty"`  // 签名密钥（可选）
	Headers map[string]string `json:"headers,omitempty"` // 自定义Header（可选）
}

// DingTalkConfig 钉钉机器人配置
type DingTalkConfig struct {
	WebhookURL string `json:"webhook_url"`
	Secret     string `json:"secret,omitempty"` // 加签密钥
}

// EmailConfig 邮件配置
type EmailConfig struct {
	SMTPHost string   `json:"smtp_host"`
	SMTPPort int      `json:"smtp_port"`
	Username string   `json:"username"`
	Password string   `json:"password"`
	To       []string `json:"to"` // 收件人列表
}

// GetFeishuAppConfig 获取飞书应用配置
func (a *AlertChannel) GetFeishuAppConfig() (*FeishuAppConfig, error) {
	var cfg FeishuAppConfig
	err := json.Unmarshal(a.Config, &cfg)
	return &cfg, err
}
