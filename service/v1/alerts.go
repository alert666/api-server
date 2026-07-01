package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"github.com/alert666/api-server/base/conf"
	"github.com/alert666/api-server/base/constant"
	"github.com/alert666/api-server/base/helper"
	"github.com/alert666/api-server/base/log"
	"github.com/alert666/api-server/base/types"
	"github.com/alert666/api-server/model"
	"github.com/alert666/api-server/pkg/email"
	"github.com/alert666/api-server/pkg/feishu"
	"github.com/alert666/api-server/store"
	"go.uber.org/zap"
)

type AlertsServicer interface {
	SendAlert(ctx context.Context, req *types.AlertReceiveReq) error
	IsSilenced(ctx context.Context, alert *types.Alert, activeSilences []*model.AlertSilence) (bool, int)
}

type CleanDuplicateFiringer interface {
	CleanDuplicateFiringAlertsTask()
	CleanRepeatIntervalAlertsTask()
}

type alertsService struct {
	cacheImpl     store.CacheStorer
	feishuImpl    feishu.Feishuer
	emailImpl     email.Emailer
	tenantKey     string
	dbTenantKey   string
	extraSyncConf *conf.ExtraSyncConf
}

func NewAlertsServicer(cache store.CacheStorer, feishuImpl feishu.Feishuer, emailImpl email.Emailer) (AlertsServicer, error) {
	e, err := conf.GetAlertExtraSync()
	if err != nil {
		return nil, err
	}

	return &alertsService{
		emailImpl:     emailImpl,
		cacheImpl:     cache,
		feishuImpl:    feishuImpl,
		tenantKey:     conf.GetAlertTenantKey(),
		dbTenantKey:   constant.AlertDBTenantKey,
		extraSyncConf: e,
	}, nil
}

func NewCleanDuplicateFiringer(cache store.CacheStorer) CleanDuplicateFiringer {
	return &alertsService{
		cacheImpl: cache,
	}
}

func (receiver *alertsService) SendAlert(ctx context.Context, req *types.AlertReceiveReq) error {
	if conf.GetAlertPrintReceivedData() {
		log.WithRequestID(ctx).Info("received alertManager body", zap.Any("data", req))
	}
	// 通过 templateName 获取告警模板（含关联的 Channel）
	alertTemplate, err := receiver.getTemplate(ctx, req.TemplateName)
	if err != nil {
		log.WithRequestID(ctx).Error("获取告警模板失败", zap.Error(err))
		return err
	}

	// 获取 tenant value
	tenantValue := req.CommonLabels[receiver.tenantKey]
	if tenantValue == "" {
		log.WithRequestID(ctx).Warn("CommonLabels 中未找到 " + receiver.tenantKey + ", 回退到 Alerts[0].Labels 获取")
		tenantValue = req.Alerts[0].Labels[receiver.tenantKey]
	}

	// 从远程获取配置
	if alertTemplate.ReceiveIdType == string(model.Remote) {
		if err := helper.GetRemoteReceive(ctx, req, tenantValue, alertTemplate); err != nil {
			return err
		}
	}

	alertTemplate = receiver.appendReceiver(ctx, tenantValue, req.ExtraSync, alertTemplate)
	if len(alertTemplate.ReceiveId) == 0 {
		log.WithRequestID(ctx).Debug("alertTemplate.ReceiveId 为空")
		return nil
	}
	alertChannel := alertTemplate.AlertChannel

	notifyReq, err := receiver.aggregatedAlarmGrouping(ctx, tenantValue, req.Alerts)
	if err != nil {
		log.WithRequestID(ctx).Error("告警分组失败", zap.Error(err))
		return err
	}
	notifyReq.TenantValue = tenantValue
	notifyReq.AlertChannel = alertChannel
	notifyReq.AlertTemplate = alertTemplate
	notifyReq.AlertReceiveReq = req

	var sendResult *types.NotifySendResult
	defer func() {
		if sendResult == nil {
			return
		}
		asyncCtx := context.WithoutCancel(ctx)
		go receiver.saveAlerts(asyncCtx, tenantValue, notifyReq, sendResult)
	}()

	// 根据告警渠道类型发送告警
	switch alertChannel.Type {
	case model.ChannelTypeEmail:
		sendResult, err = receiver.emailImpl.Notify(ctx, notifyReq)
		if err != nil {
			return err
		}
	case model.ChannelTypeFeishuApp:
		sendResult, err = receiver.feishuImpl.Notify(ctx, notifyReq)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("不支持的发送类型")
	}

	return err
}

// getTemplate 通过模板名称获取告警模板（含关联的 Channel 和已初始化的 SDK 客户端）
// getTemplate: fetch alert template with associated channel (O(1) Redis lookup by AlertChannelID)
func (receiver *alertsService) getTemplate(ctx context.Context, templateName string) (*model.AlertTemplate, error) {
	var template model.AlertTemplate
	found, err := receiver.cacheImpl.GetObject(ctx, store.AlertTemplateType, templateName, &template)
	if err != nil {
		log.WithRequestID(ctx).Error("get template from cache failed", zap.String("name", templateName), zap.Error(err))
		return nil, err
	}

	if !found {
		// cache miss: load from DB, cache template and channel separately
		tpl, err := aTemlpateStore.WithContext(ctx).Where(aTemlpateStore.Name.Eq(templateName)).First()
		if err != nil {
			return nil, fmt.Errorf("alert template %s not found", templateName)
		}

		template = *tpl
		channel, err := aChannelStore.WithContext(ctx).Where(aChannelStore.ID.Eq(template.AlertChannelID)).First()
		if err != nil {
			return nil, fmt.Errorf("channel (ID=%d) for template %s not found", template.AlertChannelID, templateName)
		}
		if *channel.Status != model.StatusEnabled {
			return nil, fmt.Errorf("channel [%s] for template %s is disabled", channel.Name, templateName)
		}
		template.AlertChannel = channel

		// init feishu SDK client
		switch channel.Type {
		case model.ChannelTypeEmail:
		case model.ChannelTypeFeishuApp:
			appid, appSecret, err := helper.VerificationAlertFeishuConfig(channel)
			if err != nil {
				return nil, err
			}
			receiver.feishuImpl.Init(channel.Name, appid, appSecret)
		default:
			return nil, fmt.Errorf("unsupported channel type: %s", channel.Type)
		}

		// cache template and channel separately
		if err := receiver.cacheImpl.SetObject(ctx, store.AlertTemplateType, template.Name, template, store.NeverExpires); err != nil {
			return nil, err
		}
		if err := receiver.cacheImpl.SetObject(ctx, store.AlertChannelType, channel.ID, channel, store.NeverExpires); err != nil {
			return nil, err
		}
		return &template, nil
	}

	// cache hit: O(1) lookup channel by AlertChannelID
	if template.AlertChannel == nil {
		var channel model.AlertChannel
		chFound, chErr := receiver.cacheImpl.GetObject(ctx, store.AlertChannelType, template.AlertChannelID, &channel)
		if chErr != nil {
			return nil, chErr
		}
		if !chFound {
			// channel cache miss: DB fallback
			chPtr, err := aChannelStore.WithContext(ctx).Where(aChannelStore.ID.Eq(template.AlertChannelID)).First()
			if err != nil {
				return nil, fmt.Errorf("channel (ID=%d) for template %s not found", template.AlertChannelID, templateName)
			}
			channel = *chPtr
			if err := receiver.cacheImpl.SetObject(ctx, store.AlertChannelType, channel.ID, channel, store.NeverExpires); err != nil {
				return nil, err
			}

			// channel cache miss means the in-memory SDK client may also be missing.
			switch channel.Type {
			case model.ChannelTypeEmail:
			case model.ChannelTypeFeishuApp:
				appid, appSecret, err := helper.VerificationAlertFeishuConfig(&channel)
				if err != nil {
					return nil, err
				}
				receiver.feishuImpl.Init(channel.Name, appid, appSecret)
			default:
				return nil, fmt.Errorf("unsupported channel type: %s", channel.Type)
			}
		}
		if *channel.Status != model.StatusEnabled {
			return nil, fmt.Errorf("channel [%s] for template %s is disabled", channel.Name, templateName)
		}
		template.AlertChannel = &channel
	}

	return &template, nil
}
func (receiver *alertsService) aggregatedAlarmGrouping(ctx context.Context, tenantValue string, alerts []*types.Alert) (*types.NotifyReq, error) {
	log.WithRequestID(ctx).Debug("告警分组", zap.String("tenant", tenantValue))
	alertLen := len(alerts)
	if alertLen == 0 {
		return nil, fmt.Errorf("alerts 为空, 告警分组失败")
	}
	var (
		tenantWhere       = fmt.Sprintf("%s = ?", receiver.dbTenantKey)
		notifyReq         = types.NewNotifyReq()
		existingHistories []*model.AlertHistory
		existingHistorMap = make(map[string]*model.AlertHistory)
		queryArgs         [][]interface{}
		resolvedAlertMap  = make(map[string]*types.Alert, alertLen)
		firingAlertMap    = make(map[string]*types.Alert, alertLen)
		silencedAlertMap  = make(map[string]*types.Alert)
		firingAlertArry   = make([]*types.Alert, 0, alertLen)
		resolvedAlertArry = make([]*types.Alert, 0, alertLen)
		activeSilences    []*model.AlertSilence
		now               = time.Now()
	)

	// 查询已经存在的告警
	for i := range alerts {
		queryArgs = append(queryArgs, []interface{}{
			alerts[i].Fingerprint,
			alerts[i].StartsAt.Truncate(time.Millisecond),
		})
	}
	if len(queryArgs) == 0 {
		return nil, fmt.Errorf("查询已存在告警时查询条件为空")
	}
	err := aHistoryStore.WithContext(ctx).
		UnderlyingDB().
		Preload("AlertSendRecord").
		Where(tenantWhere, tenantValue).
		Where("(fingerprint, starts_at) IN ?", queryArgs).
		Find(&existingHistories).Error
	if err != nil {
		return nil, err
	}

	// 从缓存中获取静默规则
	var found bool
	found, err = receiver.cacheImpl.GetObject(ctx, store.AlertSilenceType, tenantValue, &activeSilences)
	if err != nil {
		log.WithRequestID(ctx).Error("获取静默规则缓存失败", zap.Error(err))
	}

	if !found || err != nil {
		err = aSilenceStore.WithContext(ctx).
			UnderlyingDB().
			Where(tenantWhere, tenantValue).
			Where(aSilenceStore.Status.Eq(model.SilenceEnabled)).
			Where(aSilenceStore.EndsAt.Gte(now)).
			Where(aSilenceStore.StartsAt.Lte(now)).
			Find(&activeSilences).Error
		if err != nil {
			log.WithRequestID(ctx).Error("查询静默规则失败", zap.Error(err))
		} else {
			if err := receiver.cacheImpl.SetObject(ctx, store.AlertSilenceType, tenantValue, activeSilences, 1*time.Hour); err != nil {
				log.WithRequestID(ctx).Error("写入静默规则缓存失败", zap.Error(err))
			}
		}
	}

	// 转换历史记录为 Map 方便对比
	for i := range existingHistories {
		key := helper.GetAlertMapKey(existingHistories[i].Fingerprint, existingHistories[i].StartsAt)
		existingHistorMap[key] = existingHistories[i]
	}

	for i := range alerts {
		key := helper.GetAlertMapKey(alerts[i].Fingerprint, alerts[i].StartsAt)
		alerts[i].GeneratorURL = strings.ReplaceAll(alerts[i].GeneratorURL, "\\", "")
		// 在这里处理静默.如果静默保存到静默map里.然后更新数据了
		defaultResolvedEndTime := time.Now()
		// 校验静默
		isSilenced, silenceID := receiver.IsSilenced(ctx, alerts[i], activeSilences)
		if isSilenced {
			alerts[i].IsSilenced = true
			alerts[i].SilenceID = silenceID
			silencedAlertMap[key] = alerts[i]
			log.WithRequestID(ctx).Info("告警被静默", zap.String("fingerprint", alerts[i].Fingerprint), zap.Int("silenceID", silenceID))

			// 处理 time 类型默认值,防止数据库保存失败
			if alerts[i].Status == constant.AlertStatusFiring {
				alerts[i].EndsAt = nil
			} else {
				if alerts[i].EndsAt.IsZero() {
					alerts[i].EndsAt = &defaultResolvedEndTime
				}
			}

			continue
		} else {
			alerts[i].IsSilenced = false
		}

		// --- 处理 Firing 状态 ---
		if alerts[i].Status == constant.AlertStatusFiring {
			// 处理 time 类型默认值,防止数据库保存失败
			alerts[i].EndsAt = nil
			firingAlertMap[key] = alerts[i]
			firingAlertArry = append(firingAlertArry, alerts[i])
		}

		// --- 处理 Resolved 状态 ---
		if alerts[i].Status == constant.AlertStatusResolved {
			if existingHistor, ok := existingHistorMap[key]; ok {
				if existingHistor.Status == constant.AlertStatusResolved {
					delete(existingHistorMap, key)
					continue
				}
			}
			// 处理 time 类型默认值,防止数据库保存失败
			if alerts[i].EndsAt.IsZero() {
				alerts[i].EndsAt = &defaultResolvedEndTime
			}
			resolvedAlertArry = append(resolvedAlertArry, alerts[i])
			resolvedAlertMap[key] = alerts[i]
		}
	}
	notifyReq.ExistingAlertMap = existingHistorMap
	notifyReq.AlertArry.FiringAlertArry = firingAlertArry
	notifyReq.AlertArry.ResolvedAlertArry = resolvedAlertArry
	notifyReq.AlertMap.FiringAlertMap = firingAlertMap
	notifyReq.AlertMap.ResolvedAlertMap = resolvedAlertMap
	notifyReq.AlertMap.SilencedAlertMap = silencedAlertMap

	return notifyReq, nil
}

// saveAlerts 将告警记录持久化到数据库
func (receiver *alertsService) saveAlerts(ctx context.Context, tenant string, notifyReq *types.NotifyReq, sendResult *types.NotifySendResult) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			log.WithRequestID(ctx).Error("saveAlerts panic recovered",
				zap.String("tenant", tenant),
				zap.Any("panic", r),
				zap.String("stack", string(stack)),
			)
		}
	}()

	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var (
		allCreateSendRecords []*model.AlertSendRecord
		allUpdateSendRecords []*model.AlertSendRecord
		allUpdateAlerts      []*model.AlertHistory
	)

	batches := []map[string]*types.Alert{notifyReq.AlertMap.FiringAlertMap, notifyReq.AlertMap.ResolvedAlertMap}
	for _, batchMap := range batches {
		if len(batchMap) == 0 {
			continue
		}

		res := receiver.processAlerts(timeoutCtx, &processAlertsReq{
			notifyReq:       notifyReq,
			sendResult:      sendResult,
			batchMap:        batchMap,
			storeHistoryMap: notifyReq.ExistingAlertMap,
		})

		// 合并结果
		allCreateSendRecords = append(allCreateSendRecords, res.createSendRecords...)
		allUpdateSendRecords = append(allUpdateSendRecords, res.updateSendRecords...)
		allUpdateAlerts = append(allUpdateAlerts, res.updateAlerts...)
	}

	// --- 单独处理静默告警 ---
	silenceCreate, silenceUpdate := receiver.processSilencedAlerts(notifyReq)
	allUpdateAlerts = append(allUpdateAlerts, silenceUpdate...)

	// 创建 CreateSendRecords 的时候批量创建带有发送流水的告警 (Firing/Resolved)
	if len(allCreateSendRecords) > 0 {
		if err := aSendStore.WithContext(ctx).Create(allCreateSendRecords...); err != nil {
			log.WithRequestID(ctx).Error("批量创建告警历史记录失败", zap.String("tenant", tenant), zap.Error(err))
		}
	}

	// 批量创建静默告警 (只有 History)
	if len(silenceCreate) > 0 {
		if err := aHistoryStore.WithContext(ctx).Create(silenceCreate...); err != nil {
			log.WithRequestID(ctx).Error("批量创建静默告警历史失败", zap.String("tenant", tenant), zap.Error(err))
		}
	}

	// 更新发送记录 (ErrorMessage 等)
	if len(allUpdateSendRecords) > 0 {
		for _, updateSendRecord := range allUpdateSendRecords {
			upObj := model.AlertSendRecord{
				ErrorMessage: updateSendRecord.ErrorMessage,
			}
			if _, err := aSendStore.WithContext(timeoutCtx).Where(aSendStore.ID.Eq(updateSendRecord.ID)).Updates(upObj); err != nil {
				log.WithRequestID(ctx).Error("批量更新告警发送记录失败", zap.String("tenant", tenant), zap.Error(err))
				continue
			}
		}
	}

	// 更新告警历史 (状态、结束时间、静默标记等)
	if len(allUpdateAlerts) > 0 {
		for _, updateAlert := range allUpdateAlerts {
			upMap := map[string]interface{}{
				"status":           updateAlert.Status,
				"ends_at":          updateAlert.EndsAt,
				"send_count":       updateAlert.SendCount,
				"is_silenced":      updateAlert.IsSilenced,
				"alert_silence_id": updateAlert.AlertSilenceID,
			}

			log.WithRequestID(ctx).Info(
				"更新告警记录",
				zap.Int("alertHistoryID", updateAlert.ID),
				zap.String("Fingerprint", updateAlert.Fingerprint),
				zap.String("alertHistoryName", updateAlert.Alertname),
				zap.String("tenant", tenant),
			)

			if _, err := aHistoryStore.WithContext(timeoutCtx).Where(aHistoryStore.ID.Eq(updateAlert.ID)).Updates(upMap); err != nil {
				log.WithRequestID(ctx).Error("批量更新告警历史记录失败", zap.String("tenant", tenant), zap.Error(err))
				continue
			}
		}
	}

	for _, record := range allCreateSendRecords {
		// 防御性检查：防止切片中包含 nil 指针
		if record == nil {
			continue
		}

		// 直接遍历子切片，Go 会自动处理 len(record.AlertHistory) == 0 的情况
		for _, history := range record.AlertHistory {
			if history == nil {
				continue
			}

			log.WithRequestID(ctx).Info(
				"创建告警记录",
				zap.Int("alertHistoryID", history.ID),
				zap.String("Fingerprint", history.Fingerprint),
				zap.String("alertHistoryName", history.Alertname),
				zap.String("tenant", tenant),
			)
		}
	}

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
		// sharedAggRecord 聚合模式下的公共发送记录, 用于挂载所有告警历史, 利用 gorm 的关联机制一次性插入到数据库
		sharedAggRecord   *model.AlertSendRecord
		createSendRecords = make([]*model.AlertSendRecord, 0, alertsLen)
		updateSendRecords = make([]*model.AlertSendRecord, 0, alertsLen)
		updateAlerts      = make([]*model.AlertHistory, 0, alertsLen)
		updatedRecordIDs  = make(map[int]struct{}, alertsLen)
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
				if !alert.IsSilenced {
					storeHistory.IsSilenced = alert.IsSilenced
					storeHistory.AlertSilenceID = nil
				}
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
						errMsg := targetErr.Error()
						storeHistory.AlertSendRecord.ErrorMessage = &errMsg
						updateSendRecords = append(updateSendRecords, storeHistory.AlertSendRecord)
						updatedRecordIDs[recordID] = struct{}{} // 标记已更新，本 ID 下一条跳过
					}
				}
			}
			continue
		}

		// !exist 创建 AlertSendRecord 记录
		if !exist {
			alertHistory, err := types.ConvertToModel(receiver.tenantKey, alert, req.notifyReq.AlertTemplate.ID)
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

// CleanDuplicateFiringAlertsTask 定时清理任务：处理相同指纹但有多个 firing 状态的记录
func (receiver *alertsService) CleanDuplicateFiringAlertsTask() {
	start := time.Now()
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			zap.L().Error("cleanDuplicateFiringAlertsTask panic recovered",
				zap.Any("panic", r),
				zap.String("stack", string(stack)),
			)
			return
		}

		elapsed := time.Since(start).Milliseconds()
		zap.L().Debug("cleanDuplicateFiringAlertsTask 执行结束",
			zap.Int64("duration_ms", elapsed),
		)
	}()
	lockDuration := 5 * time.Minute
	ctx, cancle := context.WithTimeout(context.TODO(), 290*time.Second)
	defer cancle()

	al := store.AlertHistory.WithContext(ctx)

	ok, err := receiver.cacheImpl.SetNX(ctx, store.LockType, constant.AlertCleanDuplicateHistoryLockKey, time.Now().Unix(), lockDuration)
	if err != nil {
		zap.L().Error("[定时任务] cleanDuplicateFiringAlertsTask Redis 锁异常", zap.Error(err))
		return
	}
	defer receiver.cacheImpl.DelKey(ctx, store.LockType, constant.AlertCleanDuplicateHistoryLockKey)

	// 没抢到锁，说明其他副本正在执行，直接退出
	if !ok {
		zap.L().Debug("[定时任务] cleanDuplicateFiringAlertsTask 任务正在其他节点运行，本次跳过")
		return
	}

	zap.L().Debug("[定时任务] cleanDuplicateFiringAlertsTask 成功获取锁")

	// 1. 查询所有正在告警（ends_at 为空）且状态为 firing 的记录
	// 按照 StartsAt 降序排列，确保后续切片中索引 0 是最新的
	alertHistories, err := al.Where(
		store.AlertHistory.EndsAt.IsNull(),
		store.AlertHistory.Status.Eq("firing"),
	).Order(store.AlertHistory.StartsAt.Desc()).Find()

	if err != nil {
		zap.L().Error("[定时任务] 查询 firing 状态告警失败", zap.Error(err))
		return
	}

	if len(alertHistories) == 0 {
		return
	}

	// 2. 按 Cluster + Fingerprint 复合 Key 进行内存分组
	groupMap := make(map[string][]*model.AlertHistory)
	for i := range alertHistories {
		key := fmt.Sprintf("%s:%s", alertHistories[i].Cluster, alertHistories[i].Fingerprint)
		groupMap[key] = append(groupMap[key], alertHistories[i])
	}

	var idsToResolve []int
	now := time.Now()

	// 3. 筛选重复记录（保留每组最新的一条，其余标记为待清理）
	for key, records := range groupMap {
		if len(records) > 1 {
			// 从索引 1 开始全是旧记录（因为查询时用了 Desc 排序）
			for i := 1; i < len(records); i++ {
				idsToResolve = append(idsToResolve, records[i].ID)

				zap.L().Debug("[定时任务] 发现重复 Firing 记录",
					zap.String("key", key),
					zap.Int("old_id", records[i].ID),
					zap.Time("old_starts_at", records[i].StartsAt),
					zap.Time("latest_starts_at", records[0].StartsAt),
				)
			}
		}
	}

	// 4. 执行批量更新逻辑
	if len(idsToResolve) > 0 {
		// 分批处理，每批 500 条，防止单条 SQL 过大
		for i := 0; i < len(idsToResolve); i += 500 {
			end := i + 500
			if end > len(idsToResolve) {
				end = len(idsToResolve)
			}

			batchIDs := idsToResolve[i:end]
			_, err := al.Where(store.AlertHistory.ID.In(batchIDs...)).
				Updates(model.AlertHistory{
					Status: "resolved",
					EndsAt: &now,
				})

			if err != nil {
				zap.L().Error("[定时任务] 批量清理重复告警失败",
					zap.Error(err),
					zap.Int("batch_size", len(batchIDs)),
				)
				// 继续处理下一批，不中断任务
				continue
			}
		}
		zap.L().Info("[定时任务] 重复告警清理任务完成", zap.Int("total_resolved", len(idsToResolve)))
	}
}

// repeat_interval
func (receiver *alertsService) CleanRepeatIntervalAlertsTask() {
	start := time.Now()
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			zap.L().Error("cleanRepeatIntervalAlertsTask panic recovered",
				zap.Any("panic", r),
				zap.String("stack", string(stack)),
			)
			return
		}

		elapsed := time.Since(start).Milliseconds()
		zap.L().Debug("cleanRepeatIntervalAlertsTask 执行结束",
			zap.Int64("duration_ms", elapsed),
		)
	}()

	lockDuration := 5 * time.Minute
	ctx, cancle := context.WithTimeout(context.TODO(), 290*time.Second)
	defer cancle()

	al := store.AlertHistory.WithContext(ctx)

	ok, err := receiver.cacheImpl.SetNX(ctx, store.LockType, constant.AlertCleanRepeatIntervalHistoryLockKey, time.Now().Unix(), lockDuration)
	if err != nil {
		zap.L().Error("[定时任务] cleanRepeatIntervalAlertsTask Redis 锁异常", zap.Error(err))
		return
	}
	defer receiver.cacheImpl.DelKey(ctx, store.LockType, constant.AlertCleanRepeatIntervalHistoryLockKey)

	// 没抢到锁，说明其他副本正在执行，直接退出
	if !ok {
		zap.L().Debug("[定时任务] cleanRepeatIntervalAlertsTask 任务正在其他节点运行，本次跳过")
		return
	}

	zap.L().Debug("[定时任务] cleanRepeatIntervalAlertsTask 成功获取锁")

	// 1. 查询所有正在告警（ends_at 为空）且状态为 firing 的记录
	// 按照 StartsAt 降序排列，确保后续切片中索引 0 是最新的
	alertHistories, err := al.Where(
		store.AlertHistory.EndsAt.IsNull(),
		store.AlertHistory.Status.Eq("firing"),
	).Order(store.AlertHistory.StartsAt.Desc()).Find()

	if err != nil {
		zap.L().Error("[定时任务] cleanRepeatIntervalAlertsTask 查询 firing 状态告警失败", zap.Error(err))
		return
	}

	if len(alertHistories) == 0 {
		return
	}

	repeatInterval := conf.GetAlertRepeatInterval()
	if repeatInterval == 0 {
		return
	}

	now := time.Now()
	ids := make([]int, 0, len(alertHistories))
	for i := range alertHistories {
		aaa := alertHistories[i]
		if aaa.EndsAt != nil {
			continue
		}
		if aaa.UpdatedAt.Add(repeatInterval).Before(now) {
			ids = append(ids, aaa.ID)
		}
	}

	if len(ids) == 0 {
		zap.L().Info("[定时任务] cleanRepeatIntervalAlertsTask 本次无需要自动 Resolved 的告警")
		return
	}

	result, err := al.Where(aHistoryStore.ID.In(ids...)).Updates(map[string]interface{}{
		"status":  constant.AlertStatusResolved,
		"ends_at": now,
	})
	if err != nil {
		zap.L().Error("[定时任务] cleanRepeatIntervalAlertsTask 更新告警失败", zap.Error(err))
	} else {
		zap.L().Info("[定时任务] cleanRepeatIntervalAlertsTask 超时未接收到告警, 成功自动 Resolved 告警", zap.Int("count", int(result.RowsAffected)))
	}
}

// IsSilenced 查询告警是否为静默
func (receiver *alertsService) IsSilenced(ctx context.Context, alert *types.Alert, activeSilences []*model.AlertSilence) (bool, int) {
	if len(activeSilences) == 0 {
		return false, 0
	}

	for _, s := range activeSilences {
		// 1. 基础时间窗口过滤 (如果 SQL 已经过滤很准了，这里其实很快)
		if !alert.EndsAt.IsZero() {
			if alert.EndsAt != nil && alert.EndsAt.Before(s.StartsAt) {
				continue
			}
		}

		if alert.StartsAt.After(s.EndsAt) {
			continue
		}

		// 2. 根据 Type 进行单次逻辑判断
		switch s.Type {
		case model.SilenceTypeIdentity:
			// --- 优先级最高：指纹匹配 ---
			// 无需 Unmarshal，无需正则，直接比对字符串
			if s.Fingerprint == alert.Fingerprint {
				log.WithRequestID(ctx).Info("命中指纹静默", zap.String("fp", alert.Fingerprint))
				return true, s.ID
			}

		case model.SilenceTypeLabel:
			// --- 优先级次之：标签匹配 ---
			// 只有指纹没对上时，才会走到这里进行较重的逻辑
			if receiver.matchLabels(ctx, alert, s) {
				log.WithRequestID(ctx).Info("命中标签静默", zap.Int("silenceID", s.ID))
				return true, s.ID
			}
		}
	}

	return false, 0
}

// matchLabels 仅处理标签逻辑
func (receiver *alertsService) matchLabels(ctx context.Context, alert *types.Alert, silence *model.AlertSilence) bool {
	var matchers []model.Matcher
	if err := json.Unmarshal(silence.Matchers, &matchers); err != nil {
		log.WithRequestID(ctx).Error("序列化 matchers 失败", zap.Error(err))
		return false
	}

	for _, m := range matchers {
		alertVal, ok := alert.Labels[m.Name]
		if !ok {
			return false
		}

		switch m.Type {
		case "=":
			if alertVal != m.Value {
				return false
			}
		case "!=":
			if alertVal == m.Value {
				return false
			}
		case "=~":
			matched, err := regexp.MatchString("^("+m.Value+")$", alertVal)
			if err != nil {
				log.WithRequestID(ctx).Error("静默 =~ 正则匹配失败", zap.Error(err))
				return false
			}
			if !matched {
				return false
			}
		case "!~":
			matched, err := regexp.MatchString("^("+m.Value+")$", alertVal)
			if err != nil {
				log.WithRequestID(ctx).Error("静默 !~ 正则匹配失败", zap.Error(err))
				return false
			}
			if matched {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func (receiver *alertsService) processSilencedAlerts(notifyReq *types.NotifyReq) (createAlerts, updateAlerts []*model.AlertHistory) {
	silencedMap := notifyReq.AlertMap.SilencedAlertMap
	if len(silencedMap) == 0 {
		return
	}

	for key, alert := range silencedMap {
		storeHistory, exist := notifyReq.ExistingAlertMap[key]

		if exist {
			// 如果已存在且是 Firing，且本次被静默了
			storeHistory.EndsAt = alert.EndsAt
			storeHistory.SendCount += 1
			storeHistory.Status = alert.Status
			storeHistory.IsSilenced = true
			storeHistory.AlertSilenceID = &alert.SilenceID
			updateAlerts = append(updateAlerts, storeHistory)
		} else {
			// 新告警即被静默
			alertHistory, err := types.ConvertToModel(receiver.tenantKey, alert, notifyReq.AlertTemplate.ID)
			if err != nil {
				zap.L().Error("转换静默告警模型失败", zap.Error(err))
				continue
			}

			alertHistory.IsSilenced = true
			alertHistory.AlertSilenceID = &alert.SilenceID
			alertHistory.AlertSendRecordID = nil
			createAlerts = append(createAlerts, alertHistory)
		}
	}
	return
}

// appendReceiver 追加额外的接收者
func (receiver *alertsService) appendReceiver(ctx context.Context, tenantValue, extraSync string, alertTemplate *model.AlertTemplate) *model.AlertTemplate {

	if extraSync == "" || receiver.extraSyncConf == nil {
		return alertTemplate
	}

	e, err := receiver.extraSyncConf.GetConfig(extraSync)
	if err != nil {
		log.WithRequestID(ctx).Warn(extraSync)
		return alertTemplate
	}

	c, ok := e[tenantValue]
	if !ok {
		return alertTemplate
	}

	alertTemplate.ReceiveId = append(alertTemplate.ReceiveId, c...)
	return alertTemplate
}
