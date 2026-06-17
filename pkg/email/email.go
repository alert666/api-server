package email

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/smtp"
	"strings"
	"text/template"
	"time"

	"github.com/alert666/api-server/base/helper"
	"github.com/alert666/api-server/base/log"
	"github.com/alert666/api-server/base/types"
	"github.com/alert666/api-server/model"
	"go.uber.org/zap"
)

// Emailer 邮件发送接口
type Emailer interface {
	Notifyer
}

// Notifyer 告警通知接口
type Notifyer interface {
	Notify(ctx context.Context, notifyReq *types.NotifyReq) (*types.NotifySendResult, error)
}

// emailSender 邮件发送实现
type emailSender struct{}

// NewEmailSender 创建邮件发送器
func NewEmailSender() Emailer {
	return &emailSender{}
}

// mailConfig 邮件配置（从 channel Config 解析）
type mailConfig struct {
	SMTPHost string `json:"smtp_host"`
	SMTPPort int    `json:"smtp_port"`
	Username string `json:"username"`
	Password string `json:"password"`
	UseTLS   bool   `json:"use_tls"`
}

// Notify 发送告警邮件
func (e *emailSender) Notify(ctx context.Context, notifyReq *types.NotifyReq) (*types.NotifySendResult, error) {
	alertChannel := notifyReq.AlertChannel
	alertTemplate := notifyReq.AlertTemplate

	// 1. 解析邮件配置
	var cfg mailConfig
	if err := json.Unmarshal(alertChannel.Config, &cfg); err != nil {
		return nil, fmt.Errorf("解析邮件配置失败: %w", err)
	}
	// SMTP 默认端口
	if cfg.SMTPPort == 0 {
		cfg.SMTPPort = 25
	}

	// 2. 解析收件人列表
	var receiveIds []string
	if err := json.Unmarshal([]byte(alertTemplate.ReceiveId), &receiveIds); err != nil {
		return nil, fmt.Errorf("解析收件人列表失败: %w", err)
	}
	if len(receiveIds) == 0 {
		return nil, fmt.Errorf("收件人列表为空")
	}

	alertArry := notifyReq.AlertArry

	// 3. 聚合发送
	if *alertChannel.AggregationStatus == model.AggregationEnabled {
		var firingErr, resolvedErr error
		if len(alertArry.FiringAlertArry) > 0 {
			newReq := notifyReq.AlertReceiveReq.DeepCopy()
			newReq.Alerts = alertArry.FiringAlertArry
			firingErr = e.sendMail(ctx, cfg, receiveIds, "[告警通知]", alertTemplate.AggregationTemplate, newReq)
		}
		if len(alertArry.ResolvedAlertArry) > 0 {
			newReq := notifyReq.AlertReceiveReq.DeepCopy()
			newReq.Alerts = alertArry.ResolvedAlertArry
			resolvedErr = e.sendMail(ctx, cfg, receiveIds, "[告警恢复]", alertTemplate.AggregationTemplate, newReq)
		}
		return &types.NotifySendResult{
			AggregationSendResult: &types.AggregationSendResult{
				FiringErr:   firingErr,
				ResolvedErr: resolvedErr,
			},
		}, nil
	}

	// 4. 单条发送
	var results []*types.SingleSendResult
	for _, alert := range alertArry.FiringAlertArry {
		err := e.sendMail(ctx, cfg, receiveIds, "[告警通知]", alertTemplate.Template, alert)
		results = append(results, &types.SingleSendResult{Alert: alert, SendErr: err})
	}
	for _, alert := range alertArry.ResolvedAlertArry {
		err := e.sendMail(ctx, cfg, receiveIds, "[告警恢复]", alertTemplate.Template, alert)
		results = append(results, &types.SingleSendResult{Alert: alert, SendErr: err})
	}
	return &types.NotifySendResult{
		AggregationSendResult: nil,
		SingleSendResult:      results,
	}, nil
}

// sendMail 构建并发送一封邮件
func (e *emailSender) sendMail(ctx context.Context, cfg mailConfig, to []string, subjectSuffix string, tpl string, data interface{}) error {
	// 渲染模板
	content, err := renderTemplate(tpl, data)
	if err != nil {
		return fmt.Errorf("渲染邮件模板失败: %w", err)
	}

	subject := fmt.Sprintf("%s - %s", subjectSuffix, timeNow())
	msg := buildEmail(cfg.Username, to, subject, content)

	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)

	// 根据端口判断是否使用 TLS
	if cfg.SMTPPort == 465 {
		return sendMailTLS(addr, cfg.Username, cfg.Password, cfg.Username, to, msg)
	}

	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.SMTPHost)
	if err := smtp.SendMail(addr, auth, cfg.Username, to, msg); err != nil {
		log.WithRequestID(ctx).Error("发送邮件失败", zap.Error(err))
		return fmt.Errorf("发送邮件失败: %w", err)
	}
	return nil
}

// sendMailTLS 使用 TLS 发送邮件（端口 465）
func sendMailTLS(addr, username, password, from string, to []string, msg []byte) error {
	host := strings.Split(addr, ":")[0]
	tlsConfig := &tls.Config{ServerName: host}
	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("TLS 连接失败: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("SMTP 客户端创建失败: %w", err)
	}
	defer client.Quit()

	auth := smtp.PlainAuth("", username, password, host)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP 认证失败: %w", err)
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("SMTP MAIL FROM 失败: %w", err)
	}
	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("SMTP RCPT TO 失败: %w", err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA 失败: %w", err)
	}
	defer w.Close()
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("SMTP 写入失败: %w", err)
	}
	return nil
}

// renderTemplate 渲染 Go template
func renderTemplate(tpl string, data interface{}) (string, error) {
	tmpl, err := template.New("email").Funcs(helper.FuncMap).Parse(tpl)
	if err != nil {
		return "", fmt.Errorf("解析模板失败: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("执行模板失败: %w", err)
	}
	return buf.String(), nil
}

// buildEmail 构建符合 MIME 规范的邮件内容
func buildEmail(from string, to []string, subject, body string) []byte {
	var msg bytes.Buffer
	msg.WriteString("From: " + from + "\r\n")
	msg.WriteString("To: " + strings.Join(to, ", ") + "\r\n")
	msg.WriteString("Subject: =?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte(subject)) + "?=\r\n")
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)
	return msg.Bytes()
}

// timeNow 返回当前时间字符串
func timeNow() string {
	return time.Now().Format("2006-01-02 15:04:05")
}
