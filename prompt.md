# prompt

```go
package v1

import (
	"context"

	"github.com/alert666/api-server/base/types"
	"github.com/alert666/api-server/model"
)

type AlertTemplateServicer interface {
	CreateApi(ctx context.Context, req *types.ApiCreateRequest) error
	UpdateApi(ctx context.Context, req *types.ApiUpdateRequest) error
	DeleteApi(ctx context.Context, req *types.IDRequest) error
	QueryApi(ctx context.Context, req *types.IDRequest) (*model.Api, error)
	ListApi(ctx context.Context, pagination *types.ApiListRequest) (*types.ApiListResponse, error)
}

type AlertTemplateService struct{}

func AlertTemplateServicer() AlertTemplateServicer {
	return &AlertTemplateService{}
}
```

vscode 如何快速实现这些方法

在 VS Code 中，如果你安装了官方的 Go 扩展 (Go Team at Google)，有几种非常快的方法可以自动生成接口实现。

使用 Go: Generate Interface Stubs 命令（最推荐）, 这是最标准的方法，可以一次性生成所有方法的空壳。

按下键盘快捷键：Ctrl + Shift + P (Mac 为 Cmd + Shift + P)。输入并选择：Go: Generate Interface Stubs。在弹出的输入框中按照以下格式输入：

```go
recevicer *alertHistoryController AlertHistoryController
```

按下回车，所有方法就会自动插入到文件中

## base64

```go
type AlertTemplateCreateRequest struct {
	Name                string `json:"name" binding:"required"`
	Description         string `json:"description"`
	Template            string `json:"template" binding:"required"`
	AggregationTemplate string `json:"aggregationTemplate"`
	AlertChannelID      int    `json:"alertChannelID"`
}
```

Template AggregationTemplate 设置base64验证

## json tag

```go
type AlertChannelCreateRequest struct {
	Name              string `json:"name" binding:"required"`
	Type              string `json:"type" binding:"required"`
	Status            int    `json:"status" binding:"required"`
	AggregationStatus int    `json:"aggregationStatus" binding:"required"`
	Config            any    `json:"config" binding:"required"`
	Description       string `json:"description"`
}
```

- Name 限制长度15
- type 为 feishuApp或feishuBoot或webhook
- Status 0 或 1
- AggregationStatus 0 或 1
- Config 对象

## 缓存和数据库一致性

```go
func (recevicer *alertChannelService) UpdateChannel(ctx context.Context, req *types.AlertChannelUpdateRequest) error {
	var update bool
	sql := ac.WithContext(ctx)

	acObj, err := sql.Where(ac.ID.Eq(int(req.ID))).First()
	if err != nil {
		return err
	}

	acObj.Type = model.ChannelType(req.Type)
	acObj.Status = model.ChannelStatus(req.Status)
	acObj.AggregationStatus = model.AggregationStatus(req.AggregationStatus)

	if err := helper.VerificationAlertConfig(acObj.Name, model.ChannelType(req.Type), req.Config); err != nil {
		return err
	}

	c, err := json.Marshal(req.Config)
	if err != nil {
		return err
	}
	acObj.Config = c
	acObj.Description = req.Description
	if acObj.AlertTemplateID != req.TemplateID {
		update = true
	}
	acObj.AlertTemplateID = req.TemplateID

	store.Q.Transaction(func(tx *store.Query) error {

		if update {
			if err := recevicer.cache.DelKey(ctx, store.AlertType, acObj.Name); err != nil {
				return err
			}

			if err := recevicer.cache.SetObject(ctx, store.AlertType, acObj.Name, acObj, store.NeverExpires); err != nil {
				return err
			}
		}

		return tx.AlertChannel.WithContext(ctx).Save(acObj)
	})

	return nil
}
```

如何保证缓存和数据库数据一致性

## 是否存在逻辑错误

**主流程**

```go
package v1

import (
	"context"
	"fmt"
	"time"

	"github.com/alert666/api-server/base/conf"
	"github.com/alert666/api-server/base/constant"
	"github.com/alert666/api-server/base/helper"
	"github.com/alert666/api-server/base/log"
	"github.com/alert666/api-server/base/types"
	"github.com/alert666/api-server/model"
	"github.com/alert666/api-server/pkg/feishu"
	"github.com/alert666/api-server/store"
	"go.uber.org/zap"
)

type AlertsServicer interface {
	SendAlert(ctx context.Context, req *types.AlertReceiveReq) error
}

type alertsService struct {
	aggregation bool
	cache       store.CacheStorer
	feishuImpl  feishu.Feishuer
}

func NewAlertsServicer(cache store.CacheStorer, feishuImpl feishu.Feishuer) AlertsServicer {
	return &alertsService{
		cache:       cache,
		aggregation: conf.GetAlertAggregation(),
		feishuImpl:  feishuImpl,
	}
}

func (receiver *alertsService) SendAlert(ctx context.Context, req *types.AlertReceiveReq) error {
	// 获取告警发送Channel
	alertChannel, err := receiver.getChannel(ctx, req.ChannelName)
	if err != nil {
		log.WithRequestID(ctx).Error("获取告警发送channel失败", zap.Error(err))
		return err
	}

	if alertChannel.AlertTemplate == nil {
		return fmt.Errorf("%s alertChannel 未绑定模板, 发送告警失败", alertChannel.Name)
	}

	var firingAlerts, resolvedAlerts []*types.Alert
	firingAlerts, resolvedAlerts = receiver.aggregatedAlarmGrouping(ctx, req.Alerts)
	notifyReq := &types.NotifyReq{
		AlertChannel: alertChannel,
		NotifyAlerts: &types.NotifyAlerts{
			FiringAlerts:   firingAlerts,
			ResolvedAlerts: resolvedAlerts,
		},
		AlertReceiveReq: req,
	}

	var sendResult *types.NotifySendResult
	switch alertChannel.Type {
	case model.ChannelTypeFeishuApp:
		sendResult, err = receiver.feishuImpl.Notify(ctx, notifyReq)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("不支持的发送类型")
	}

	if sendResult != nil {
		go receiver.saveAlerts(ctx, notifyReq, sendResult)
	}
	return nil
}

// getChannel 获取告警发送方式
func (receiver *alertsService) getChannel(ctx context.Context, channelName string) (*model.AlertChannel, error) {
	var channel model.AlertChannel
	found, err := receiver.cache.GetObject(ctx, store.AlertType, channelName, &channel)
	if err != nil {
		zap.L().Error("从缓存获取渠道失败", zap.String("name", channelName), zap.Error(err))
		return nil, err
	}

	if !found {
		channel, err := ac.WithContext(ctx).Preload(ac.AlertTemplate).Where(ac.Name.Eq(channelName)).First()
		if err != nil {
			return nil, err
		}
		if *channel.Status != model.StatusEnabled {
			return nil, fmt.Errorf("告警通道 %s 未启用, 发送失败", channel.Name)
		}

		switch channel.Type {
		case model.ChannelTypeFeishuApp:
			appid, appSecret, err := helper.VerificationAlertFeishuConfig(channel)
			if err != nil {
				return nil, err
			}
			// 缓存客户端
			receiver.feishuImpl.Init(channel.Name, appid, appSecret)
			// 缓存 redis
			if err := receiver.cache.SetObject(ctx, store.AlertType, channel.Name, channel, store.NeverExpires); err != nil {
				return nil, err
			}
			return channel, nil
		default:
			return nil, fmt.Errorf("不支持的 Channel 类型: %s", channel.Type)
		}
	}

	return &channel, nil
}

// aggregatedAlarmGrouping 聚合告警分组
// 如果通知为聚合告警时, 需要将告警分配 firing 和 resolved 两组, 分别发送
func (receiver *alertsService) aggregatedAlarmGrouping(ctx context.Context, alerts []*types.Alert) (firingAlerts, resolvedAlerts []*types.Alert) {
	if len(alerts) == 0 {
		return
	}

	firingAlerts = make([]*types.Alert, 0)
	resolvedAlerts = make([]*types.Alert, 0)
	for i := range alerts {
		if alerts[i].Status == constant.AlertStatusFiring {
			// 告警逻辑
			// TODO 判断告警静默,如果静默continue
			silence, err := receiver.getSilence(ctx, alerts[i])
			if err != nil {
				zap.L().Error("判断静默失败", zap.Error(err))
				continue
			}

			if silence {
				continue
			}
			// 如果是 Firing 那么将 EndsAt 设置为 nil
			alerts[i].EndsAt = nil
			firingAlerts = append(firingAlerts, alerts[i])
		}

		if alerts[i].Status == constant.AlertStatusResolved {
			resolvedAlerts = append(resolvedAlerts, alerts[i])
		}
	}
	return
}

// TODO getSilence
func (receiver *alertsService) getSilence(ctx context.Context, alert *types.Alert) (silence bool, err error) {
	return silence, nil
}

// saveAlerts 将告警记录持久化到数据库
func (receiver *alertsService) saveAlerts(ginCtx context.Context, notifyReq *types.NotifyReq, sendResult *types.NotifySendResult) {
	log.WithRequestID(ginCtx).Info("告警数据开始持久化")
	alerts := notifyReq.AlertReceiveReq.Alerts

	// saveAlerts := make([]*model.AlertHistory, 0, len(alerts))
	// 数据库查询条件, 查询本次告警的所有数据
	var queryArgs [][]interface{}
	for _, a := range alerts {
		queryArgs = append(queryArgs, []interface{}{
			a.Fingerprint,
			a.StartsAt.Truncate(time.Millisecond),
		})
	}

	timeoutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var existingHistories []*model.AlertHistory
	err := al.WithContext(timeoutCtx).
		UnderlyingDB().
		Preload("AlertSendRecord").
		Where("(fingerprint, starts_at) IN ?", queryArgs).
		Find(&existingHistories).Error

	if err != nil {
		log.WithRequestID(ginCtx).Error("批量查询告警历史失败, 协程退出", zap.Error(err))
		return
	}

	if len(existingHistories) == 0 {
		log.WithRequestID(ginCtx).Error("未查询到告警数据, 协程退出")
		return
	}

	var (
		aggregationStatus     = *notifyReq.AlertChannel.AggregationStatus
		aggregationSendResult = sendResult.AggregationSendResult
		createSendStatus      = true
		sendRecordID          int
	)
	// 转换单次发送告警记录的发送状态
	var singleSendResult map[string]error
	if aggregationStatus == model.AggregationDisabled {
		singleSendResult = make(map[string]error, len(sendResult.SingleSendResult))
		for i := range sendResult.SingleSendResult {
			key := helper.GetAlertMapKey(sendResult.SingleSendResult[i].Alert.Fingerprint, sendResult.SingleSendResult[i].Alert.StartsAt)
			singleSendResult[key] = sendResult.SingleSendResult[i].SendErr
		}
	}

	// 将查出的结果转成 map 方便快速比对
	storeHistoryMap := make(map[string]*model.AlertHistory)
	for exisIndex := range existingHistories {
		currAlert := existingHistories[exisIndex]
		// key 使用指纹 + 时间戳(纳秒)
		key := helper.GetAlertMapKey(currAlert.Fingerprint, currAlert.StartsAt)
		storeHistoryMap[key] = currAlert
	}

	// alertHistory 没有查询到的记录, 说明是新告警, 需要创建记录，存储需要创建的模型
	createAlerts := make([]*model.AlertHistory, 0, len(alerts)/2)
	// alertHistory 查询到的记录, 说明是已存在的告警, 需要更新 EndsAt 和 Status 字段，存储需要更新的模型
	updateAlerts := make([]*model.AlertHistory, 0, len(alerts))
	updateSendRecords := make([]*model.AlertSendRecord, 0, len(alerts))

	firingAlertsMap, resolvedAlertsMap := receiver.getAlertsMap(notifyReq.NotifyAlerts)
	for key, firingAlert := range firingAlertsMap {
		storeHistory, exist := storeHistoryMap[key]
		// exist 已存在记录, 说明是重复告警, 只需要将发送次数加 1 即可
		if exist {
			storeHistory.SendCount += 1
			updateAlerts = append(updateAlerts, storeHistory)

			if aggregationStatus == model.AggregationEnabled {
				storeHistory.AlertSendRecord.ErrorMessage += aggregationSendResult.FiringErr.Error()
			} else {
				singleErr := singleSendResult[key]
				storeHistory.AlertSendRecord.ErrorMessage += singleErr.Error()
			}
			updateSendRecords = append(updateSendRecords, storeHistory.AlertSendRecord)
			return
		}

		// !exist 创建 AlertSendRecord 记录
		if aggregationStatus == model.AggregationEnabled {
			sendRecord := &model.AlertSendRecord{}
			model.UpdateSendRecordStatus(sendRecord, aggregationSendResult.FiringErr)
			if createSendStatus {
				err := as.WithContext(timeoutCtx).Create(sendRecord)
				if err != nil {
					log.WithRequestID(ginCtx).Error("创建聚合告警发送记录失败", zap.Error(err))
					continue
				}
				createSendStatus = false
				sendRecordID = sendRecord.ID
			}
		}

		if aggregationStatus == model.AggregationDisabled {
			sendRecord := &model.AlertSendRecord{}
			singleErr := singleSendResult[key]
			model.UpdateSendRecordStatus(sendRecord, singleErr)
			err := as.WithContext(timeoutCtx).Create(sendRecord)
			if err != nil {
				log.WithRequestID(ginCtx).Error("创建非聚合告警发送记录失败", zap.Error(err))
				continue
			}
			sendRecordID = sendRecord.ID
		}

		// !exist 创建 AlertHistory 记录
		alertHistory, err := types.ConvertToModel(firingAlert, notifyReq.AlertChannel.ID)
		if err != nil {
			zap.L().Error("转换告警模型失败", zap.Error(err))
			continue
		}
		alertHistory.AlertSendRecordID = sendRecordID
		createAlerts = append(createAlerts, alertHistory)
	}

	// 循环 resolvedAlertsMap, 如果 alertHistory 中存在记录, 说明是已存在的告警, 需要更新 EndsAt 和 Status 字段
	for key, resolvedAlert := range resolvedAlertsMap {
		storeHistory, exist := storeHistoryMap[key]
		// 发送次数加 1
		storeHistory.SendCount += 1
		if exist {
			// 已存在记录, 更新 EndsAt 和 Status 字段
			storeHistory.EndsAt = resolvedAlert.EndsAt
			storeHistory.Status = resolvedAlert.Status
			updateAlerts = append(updateAlerts, storeHistory)
		}
		if aggregationStatus == model.AggregationEnabled {
			storeHistory.AlertSendRecord.ErrorMessage += aggregationSendResult.FiringErr.Error()
		}
		if aggregationStatus == model.AggregationDisabled {
			storeHistory.AlertSendRecord.ErrorMessage += singleSendResult[key].Error()
		}
		updateSendRecords = append(updateSendRecords, storeHistory.AlertSendRecord)
	}

	// 批量创建和更新
	if len(createAlerts) > 0 {
		if err := al.WithContext(timeoutCtx).Create(createAlerts...); err != nil {
			log.WithRequestID(ginCtx).Error("批量创建告警历史记录失败", zap.Error(err), zap.Any("data", createAlerts))
		}
	}
	if len(updateAlerts) > 0 {
		for _, updateAlert := range updateAlerts {
			upObj := model.AlertHistory{
				Status: updateAlert.Status,
				EndsAt: updateAlert.EndsAt,
			}
			if _, err := al.WithContext(timeoutCtx).Where(al.ID.Eq(updateAlert.ID)).Updates(upObj); err != nil {
				log.WithRequestID(ginCtx).Error("批量更新告警历史记录失败", zap.Error(err))
				continue
			}
		}
	}
	if len(updateSendRecords) > 0 {
		for _, updateSendRecord := range updateSendRecords {
			upObj := model.AlertSendRecord{
				ErrorMessage: updateSendRecord.ErrorMessage,
			}
			if _, err := as.WithContext(timeoutCtx).Where(as.ID.Eq(updateSendRecord.ID)).Updates(upObj); err != nil {
				log.WithRequestID(ginCtx).Error("批量更新告警发送记录失败", zap.Error(err))
				continue
			}
		}
	}
}

func (receiver *alertsService) getAlertsMap(notifyAlerts *types.NotifyAlerts) (firingAlertsMap, resolvedAlertsMap map[string]*types.Alert) {
	f := notifyAlerts.FiringAlerts
	r := notifyAlerts.ResolvedAlerts
	firingAlertsMap = make(map[string]*types.Alert, len(f))
	resolvedAlertsMap = make(map[string]*types.Alert, len(r))

	for i := range f {
		key := helper.GetAlertMapKey(f[i].Fingerprint, f[i].StartsAt)
		firingAlertsMap[key] = notifyAlerts.FiringAlerts[i]
	}

	for i := range r {
		key := helper.GetAlertMapKey(r[i].Fingerprint, r[i].StartsAt)
		resolvedAlertsMap[key] = r[i]
	}

	return firingAlertsMap, resolvedAlertsMap
}

// TODO 处理相同指纹但是有多个触发时间正在告警的记录
// 将最早的告警记录的 EndsAt 和 Status 字段更新
func (receiver *alertsService) DisableBefoceAlertHistory(ctx context.Context, cluster string) map[string][]*model.AlertHistory {
	al := store.AlertHistory.WithContext(ctx)

	alertHistorys, err := al.Where(store.AlertHistory.Cluster.Eq(cluster)).Where(store.AlertHistory.EndsAt.IsNull()).Find()
	if err != nil {
		zap.L().Error("查询旧的 alertHistory 失败", zap.Error(err))
		return nil
	}
	alertHistoryMap := make(map[string][]*model.AlertHistory, len(alertHistorys))
	for i := range alertHistorys {
		fp := alertHistorys[i].Fingerprint
		alertHistoryMap[fp] = append(alertHistoryMap[fp], alertHistorys[i])
	}

	return alertHistoryMap
}
```

**模型定义**

```go
package model

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// AlertHistory 告警历史记录表
type AlertHistory struct {
	ID                int              `gorm:"primaryKey;autoIncrement;comment:主键ID"`
	Cluster           string           `gorm:"type:varchar(128);index;comment:租户"`
	CreatedAt         time.Time        `gorm:"type:datetime;autoCreateTime;comment:本条记录存入数据库的时间"`
	Fingerprint       string           `gorm:"type:varchar(128);not null;uniqueIndex:idx_alert_unique;comment:指纹"`
	StartsAt          time.Time        `gorm:"type:datetime(3);precision:3;not null;uniqueIndex:idx_alert_unique;comment:开始时间"`
	EndsAt            *time.Time       `gorm:"type:datetime;index:idx_ends_at;comment:告警恢复时间(未恢复则为NULL)"`
	Status            string           `gorm:"type:varchar(32);not null;index;comment:告警状态(如: firing, resolved)"`
	Alertname         string           `gorm:"type:varchar(255);not null;index;comment:告警名称"`
	Severity          string           `gorm:"type:varchar(32);index;comment:告警级别(如: critical, warning, info)"`
	Instance          string           `gorm:"type:varchar(255);index;comment:告警发生的实例(如IP或主机名)"`
	Labels            datatypes.JSON   `gorm:"type:json;comment:告警标签集合"`
	Annotations       datatypes.JSON   `gorm:"type:json;comment:告警详情/注解"`
	AlertChannelID    int              `gorm:"not null;index;comment:关联的告警发送通道ID"`
	AlertChannel      *AlertChannel    `gorm:"foreignKey:AlertChannelID" json:"alertChannel"`
	AlertSendRecordID int              `gorm:"not null;index;comment:关联的告警发送记录ID"`
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
	ID                int             `gorm:"primaryKey;autoIncrement"`
	CreatedAt         time.Time       `gorm:"column:created_at" json:"createdAt,omitempty"`
	UpdatedAt         time.Time       `gorm:"column:updated_at" json:"updatedAt,omitempty"`
	AlertHistory      []*AlertHistory `gorm:"foreignKey:ID" json:"alertHistory"`
	SendStatus        string          `gorm:"type:varchar(32);not null;index;comment:发送状态(success, failed)" json:"sendStatus"`
	ErrorMessage      string          `gorm:"type:text;comment:如果发送失败，记录失败的报错详情(供排查)"`
	ExternalMessageID string          `gorm:"type:varchar(255);index;comment:第三方平台返回的消息ID(如飞书的 message_id)" json:"externalMessageID"`
}

func (*AlertSendRecord) TableName() string {
	return "alert_send_records"
}

func UpdateSendRecordStatus(record *AlertSendRecord, sendErr error) {
	if sendErr == nil {
		record.SendStatus = AlertSendRecordStatusSuccess
	} else {
		record.SendStatus = AlertSendRecordStatusFailed
		record.ErrorMessage = fmt.Sprintf("record.ErrorMessage \n%s", sendErr.Error())
	}
}
```

## 优化 alertSendRecord 创建

```go
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
	ID                int             `gorm:"primaryKey;autoIncrement"`
	CreatedAt         time.Time       `gorm:"column:created_at" json:"createdAt,omitempty"`
	UpdatedAt         time.Time       `gorm:"column:updated_at" json:"updatedAt,omitempty"`
	AlertHistory      []*AlertHistory `gorm:"foreignKey:ID" json:"alertHistory"`
	SendStatus        string          `gorm:"column:send_status;type:varchar(32);not null;index;comment:发送状态(success, failed)" json:"sendStatus"`
	ErrorMessage      string          `gorm:"column:error_message;type:text;comment:如果发送失败，记录失败的报错详情(供排查)"`
	ExternalMessageID string          `gorm:"column:external_message_id;type:varchar(255);index;comment:第三方平台返回的消息ID(如飞书的 message_id)" json:"externalMessageID"`
}

func (*AlertSendRecord) TableName() string {
	return "alert_send_records"
}

func UpdateSendRecordStatus(record *AlertSendRecord, sendErr error) {
	if sendErr == nil {
		record.SendStatus = AlertSendRecordStatusSuccess
	} else {
		record.SendStatus = AlertSendRecordStatusFailed
		record.ErrorMessage = fmt.Sprintf("record.ErrorMessage \n%s", sendErr.Error())
	}
}

// !exist 创建 AlertSendRecord 记录
		if !exist {
			if aggregationStatus == model.AggregationEnabled {
				sendRecord := &model.AlertSendRecord{}
				model.UpdateSendRecordStatus(sendRecord, aggregationSendResult.FiringErr)
				if createSendStatus {
					err := as.WithContext(ctx).Create(sendRecord)
					if err != nil {
						log.WithRequestID(ctx).Error("创建聚合告警发送记录失败", zap.Error(err))
						continue
					}
					createSendStatus = false
					sendRecordID = sendRecord.ID
				}
			}

			if aggregationStatus == model.AggregationDisabled {
				sendRecord := &model.AlertSendRecord{}
				singleErr := singleSendResult[key]
				model.UpdateSendRecordStatus(sendRecord, singleErr)
				err := as.WithContext(ctx).Create(sendRecord)
				if err != nil {
					log.WithRequestID(ctx).Error("创建非聚合告警发送记录失败", zap.Error(err), zap.String("status", alert.Status))
					continue
				}
				sendRecordID = sendRecord.ID
			}

			// !exist 创建 AlertHistory 记录
			alertHistory, err := types.ConvertToModel(alert, notifyReq.AlertChannel.ID)
			if err != nil {
				zap.L().Error("转换告警模型失败", zap.Error(err), zap.Any("data", alertHistory))
				continue
			}
			alertHistory.AlertSendRecordID = sendRecordID
			createAlerts = append(createAlerts, alertHistory)
		}
```

我现在 AlertSendRecord AlertHistory 是关联的，我想在不存在告警记录的时候根据aggregationStatus的值，去创建多个或单个AlertSendRecord，然后将需要创建的AlertHistory赋值给AlertSendRecord.AlertHistory然后一次性去创建？
