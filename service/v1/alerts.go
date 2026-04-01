package v1

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/qinquanliuxiang666/alertmanager/base/conf"
	"github.com/qinquanliuxiang666/alertmanager/base/constant"
	"github.com/qinquanliuxiang666/alertmanager/base/helper"
	"github.com/qinquanliuxiang666/alertmanager/base/log"
	"github.com/qinquanliuxiang666/alertmanager/base/types"
	"github.com/qinquanliuxiang666/alertmanager/model"
	"github.com/qinquanliuxiang666/alertmanager/pkg/feishu"
	"github.com/qinquanliuxiang666/alertmanager/store"
	"go.uber.org/zap"
)

type AlertsServicer interface {
	SendAlert(ctx context.Context, req *types.AlertReceiveReq) error
}

type alertsService struct {
	cache       store.CacheStorer
	feishuImpl  feishu.Feishuer
	tenantKey   string
	dbTenantKey string
}

func NewAlertsServicer(cache store.CacheStorer, feishuImpl feishu.Feishuer) AlertsServicer {
	return &alertsService{
		cache:       cache,
		feishuImpl:  feishuImpl,
		tenantKey:   conf.GetAlertTenantKey(),
		dbTenantKey: constant.AlertDBTenantKey,
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

	log.WithRequestID(ctx).Info("告警分组完成, 开始发送告警", zap.Int("firingAlerts", len(firingAlerts)), zap.Int("resolvedAlerts", len(resolvedAlerts)))
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
		asyncCtx := context.WithoutCancel(ctx)
		go receiver.saveAlerts(asyncCtx, notifyReq, sendResult)
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
func (receiver *alertsService) saveAlerts(ctx context.Context, notifyReq *types.NotifyReq, sendResult *types.NotifySendResult) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			zap.L().Error("saveAlerts panic recovered",
				zap.Any("panic", r),
				zap.String("stack", string(stack)), // 这行会告诉你具体是代码哪一行崩了
			)
		}
	}()

	log.WithRequestID(ctx).Info("告警数据开始持久化")

	var (
		alerts = notifyReq.AlertReceiveReq.Alerts
		// 数据库查询条件, 查询本次告警的所有数据
		queryArgs [][]interface{}
	)

	// 告警数据里 tenantKey 对应的值, 用于后续查询数据库时区分租户, 这里默认取第一条告警数据的 tenantKey 值作为本次查询的值, 因为同一批次的告警数据应该是同一个租户的
	var tenantValue string
	for i, a := range alerts {
		if i == 0 {
			tenantValue = a.Labels[receiver.tenantKey]
		}
		queryArgs = append(queryArgs, []interface{}{
			a.Fingerprint,
			a.StartsAt.Truncate(time.Millisecond),
		})
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// TODO 租户逻辑
	var existingHistories []*model.AlertHistory
	tenantWhere := fmt.Sprintf("%s = ?", receiver.dbTenantKey)
	err := al.WithContext(timeoutCtx).
		UnderlyingDB().
		Preload("AlertSendRecord").
		Where(tenantWhere, tenantValue).
		Where("(fingerprint, starts_at) IN ?", queryArgs).
		Find(&existingHistories).Error
	if err != nil {
		log.WithRequestID(ctx).Error("批量查询告警历史失败, 协程退出", zap.Error(err))
		return
	}
	log.WithRequestID(ctx).Debug("查询告警数据成功, 开始处理数据")

	// 将查出的结果转成 map 方便快速比对
	storeHistoryMap := make(map[string]*model.AlertHistory)
	for exisIndex := range existingHistories {
		currAlert := existingHistories[exisIndex]
		// key 使用指纹 + 时间戳(纳秒)
		key := helper.GetAlertMapKey(currAlert.Fingerprint, currAlert.StartsAt)
		storeHistoryMap[key] = currAlert
	}

	firingAlertsMap, resolvedAlertsMap := receiver.getAlertsMap(notifyReq.NotifyAlerts)

	var (
		allCreateSendRecords []*model.AlertSendRecord
		allUpdateSendRecords []*model.AlertSendRecord
		allUpdateAlerts      []*model.AlertHistory
	)

	log.WithRequestID(ctx).Debug("告警记录处理完成, 开始批量持久化")

	var sharedAggRecord []*model.AlertSendRecord
	batches := []map[string]*types.Alert{firingAlertsMap, resolvedAlertsMap}
	for _, batchMap := range batches {
		if len(batchMap) == 0 {
			continue
		}

		res := receiver.processAlerts(timeoutCtx, &processAlertsReq{
			notifyReq:       notifyReq,
			sendResult:      sendResult,
			batchMap:        batchMap,
			storeHistoryMap: storeHistoryMap,
		})

		// 合并结果
		allCreateSendRecords = append(allCreateSendRecords, res.createSendRecords...)
		allUpdateSendRecords = append(allUpdateSendRecords, res.updateSendRecords...)
		allUpdateAlerts = append(allUpdateAlerts, res.updateAlerts...)
		sharedAggRecord = append(sharedAggRecord, res.sharedAggRecord)
	}

	// 批量创建和更新
	if len(allCreateSendRecords) > 0 {
		zap.L().Debug("批量创建告警历史记录")
		if err := as.WithContext(ctx).Create(allCreateSendRecords...); err != nil {
			log.WithRequestID(ctx).Error("批量创建告警历史记录失败", zap.Error(err))
		}
	}

	if len(allUpdateSendRecords) > 0 {
		zap.L().Debug("更新告警发送记录")
		for _, updateSendRecord := range allUpdateSendRecords {
			upObj := model.AlertSendRecord{
				ErrorMessage: updateSendRecord.ErrorMessage,
			}
			if _, err := as.WithContext(timeoutCtx).Where(as.ID.Eq(updateSendRecord.ID)).Updates(upObj); err != nil {
				log.WithRequestID(ctx).Error("批量更新告警发送记录失败", zap.Error(err))
				continue
			}
		}
	}
	if len(allUpdateAlerts) > 0 {
		for _, updateAlert := range allUpdateAlerts {
			zap.L().Debug("更新告警历史记录")
			upMap := map[string]interface{}{
				"status":     updateAlert.Status,
				"ends_at":    updateAlert.EndsAt,
				"send_count": updateAlert.SendCount,
			}
			if _, err := al.WithContext(timeoutCtx).Where(al.ID.Eq(updateAlert.ID)).Updates(upMap); err != nil {
				log.WithRequestID(ctx).Error("批量更新告警历史记录失败", zap.Error(err))
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

type processAlertsReq struct {
	notifyReq       *types.NotifyReq
	sendResult      *types.NotifySendResult
	batchMap        map[string]*types.Alert        // 当前批次的告警数据，key 是指纹+时间戳
	storeHistoryMap map[string]*model.AlertHistory // 数据库中已存在的告警历史记录，key 是指纹+时间戳
}

type processAlertsResult struct {
	createSendRecords []*model.AlertSendRecord
	updateSendRecords []*model.AlertSendRecord
	updateAlerts      []*model.AlertHistory
	sharedAggRecord   *model.AlertSendRecord
}

func (receiver *alertsService) processAlerts(ctx context.Context, req *processAlertsReq) (result *processAlertsResult) {
	var (
		alertsLen             = len(req.notifyReq.AlertReceiveReq.Alerts)
		aggregationStatus     = *req.notifyReq.AlertChannel.AggregationStatus
		aggregationSendResult = req.sendResult.AggregationSendResult
		singleSendResult      map[string]error
		sharedAggRecord       *model.AlertSendRecord
		createSendRecords     = make([]*model.AlertSendRecord, 0, alertsLen)
		updateSendRecords     = make([]*model.AlertSendRecord, 0, alertsLen)
		updateAlerts          = make([]*model.AlertHistory, 0, alertsLen)
		updatedRecordIDs      = make(map[int]struct{}, alertsLen)
	)

	// 转换单次发送告警记录的发送状态
	if aggregationStatus == model.AggregationDisabled {
		singleSendResult = make(map[string]error, len(req.sendResult.SingleSendResult))
		for i := range req.sendResult.SingleSendResult {
			key := helper.GetAlertMapKey(req.sendResult.SingleSendResult[i].Alert.Fingerprint, req.sendResult.SingleSendResult[i].Alert.StartsAt)
			singleSendResult[key] = req.sendResult.SingleSendResult[i].SendErr
		}
	}

	// 如果是聚合模式，准备一个公共的 Record
	if aggregationStatus == model.AggregationEnabled && len(req.batchMap) > 0 {
		var batchErr error
		if aggregationSendResult != nil {
			// 随便看一眼 Map 里的第一个元素，决定当前是处理 Firing 还是 Resolved 批次
			for _, alert := range req.batchMap {
				if alert.Status == constant.AlertStatusResolved {
					batchErr = aggregationSendResult.ResolvedErr
				} else {
					batchErr = aggregationSendResult.FiringErr
				}
				break
			}
		}
		// 初始化聚合容器
		sharedAggRecord = model.UpdateSendRecordStatus(batchErr)
		sharedAggRecord.AlertHistory = make([]*model.AlertHistory, 0, alertsLen)
	}

	for key, alert := range req.batchMap {
		// exist 已存在记录, 说明是重复告警, 只需要将发送次数加 1 即可, 进行下一次循环
		storeHistory, exist := req.storeHistoryMap[key]
		if exist {
			storeHistory.SendCount += 1
			// 已存在记录并且为 Resolved, 更新 EndsAt 和 Status 字段
			if alert.Status == constant.AlertStatusResolved {
				storeHistory.EndsAt = alert.EndsAt
				storeHistory.Status = alert.Status
			}
			if alert.Status == constant.AlertStatusFiring {
				storeHistory.EndsAt = nil
				storeHistory.Status = alert.Status
			}
			// 将修改后的 alertHistory 追加到更新的数组中
			updateAlerts = append(updateAlerts, storeHistory)

			// 处理已存在记录的发送状态更新
			if storeHistory.AlertSendRecord != nil {
				recordID := storeHistory.AlertSendRecord.ID
				if _, seen := updatedRecordIDs[recordID]; !seen {
					// 这里的逻辑依然动态根据 alert.Status 决定记录哪个 Err
					var targetErr error
					if aggregationSendResult != nil {
						if alert.Status == constant.AlertStatusResolved {
							targetErr = aggregationSendResult.ResolvedErr
						} else {
							targetErr = aggregationSendResult.FiringErr
						}
					}

					if targetErr != nil {
						storeHistory.AlertSendRecord.ErrorMessage += "\n" + targetErr.Error()
						updateSendRecords = append(updateSendRecords, storeHistory.AlertSendRecord)
						updatedRecordIDs[recordID] = struct{}{} // 标记已更新，本 ID 下一条跳过
					}
				}
			}
			continue
		}

		// !exist 创建 AlertSendRecord 记录
		if !exist {
			alertHistory, err := types.ConvertToModel(receiver.tenantKey, alert, req.notifyReq.AlertChannel.ID)
			if err != nil {
				log.WithRequestID(ctx).Error("转换告警模型失败", zap.Error(err), zap.Any("data", alertHistory))
				continue
			}

			if aggregationStatus == model.AggregationEnabled {
				// 修正：无论标志位如何，所有新产生的告警历史都必须挂载
				sharedAggRecord.AlertHistory = append(sharedAggRecord.AlertHistory, alertHistory)
			} else {
				// 非聚合模式处理每一条
				singleErr := singleSendResult[key]
				sendRecord := model.UpdateSendRecordStatus(singleErr)
				sendRecord.AlertHistory = []*model.AlertHistory{alertHistory}
				createSendRecords = append(createSendRecords, sendRecord)
			}

		}
	}

	// 防止 nil 指针
	if aggregationStatus == model.AggregationEnabled && sharedAggRecord != nil && len(sharedAggRecord.AlertHistory) > 0 {
		createSendRecords = append(createSendRecords, sharedAggRecord)
	}

	return &processAlertsResult{
		createSendRecords: createSendRecords,
		updateSendRecords: updateSendRecords,
		updateAlerts:      updateAlerts,
		sharedAggRecord:   sharedAggRecord,
	}
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
