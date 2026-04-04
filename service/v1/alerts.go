package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"runtime/debug"
	"strings"
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

type CleanDuplicateFiringer interface {
	CleanDuplicateFiringAlertsTask()
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

func NewCleanDuplicateFiringer(cache store.CacheStorer) CleanDuplicateFiringer {
	return &alertsService{
		cache: cache,
	}
}

func (receiver *alertsService) SendAlert(ctx context.Context, req *types.AlertReceiveReq) error {
	log.WithRequestID(ctx).Debug("接收告警数据", zap.Any("data", req))
	// 获取告警发送Channel
	alertChannel, err := receiver.getChannel(ctx, req.ChannelName)
	if err != nil {
		log.WithRequestID(ctx).Error("获取告警发送channel失败", zap.Error(err))
		return err
	}

	if alertChannel.AlertTemplate == nil {
		return fmt.Errorf("%s alertChannel 未绑定模板, 发送告警失败", alertChannel.Name)
	}

	tenantValue := req.Alerts[0].Labels[receiver.tenantKey]
	notifyReq, err := receiver.aggregatedAlarmGrouping(ctx, tenantValue, req.Alerts)
	if err != nil {
		log.WithRequestID(ctx).Error("告警分组失败", zap.Error(err))
		return err
	}
	notifyReq.TenantValue = tenantValue
	notifyReq.AlertChannel = alertChannel
	notifyReq.AlertReceiveReq = req

	log.WithRequestID(ctx).Info("通过告警通道发送告警", zap.Any("firingAlertArry", notifyReq.AlertArry.FiringAlertArry), zap.Any("resolvedAlertArry", notifyReq.AlertArry.ResolvedAlertArry))
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

	log.WithRequestID(ctx).Info("持久化告警数据", zap.String("channelName", alertChannel.Name))
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
		channel, err := aChannel.WithContext(ctx).Preload(aChannel.AlertTemplate).Where(aChannel.Name.Eq(channelName)).First()
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
func (receiver *alertsService) aggregatedAlarmGrouping(ctx context.Context, tenantValue string, alerts []*types.Alert) (*types.NotifyReq, error) {
	log.WithRequestID(ctx).Debug("聚合告警分组", zap.String("tenantValue", tenantValue))
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
	err := aHistory.WithContext(ctx).
		UnderlyingDB().
		Preload("AlertSendRecord").
		Where(tenantWhere, tenantValue).
		Where("(fingerprint, starts_at) IN ?", queryArgs).
		Find(&existingHistories).Error
	if err != nil {
		return nil, err
	}

	// 查询静默规则
	err = aSilence.WithContext(ctx).
		UnderlyingDB().
		Where(tenantWhere, tenantValue).
		Where(aSilence.Status.Eq(model.SilenceEnabled)).
		Where(aSilence.StartsAt.Lte(now)).
		Where(aSilence.EndsAt.Gte(now)).
		Find(&activeSilences).Error
	if err != nil {
		zap.L().Error("查询静默规则失败", zap.Error(err))
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
		// --- 处理 Firing 状态 ---
		if alerts[i].Status == constant.AlertStatusFiring {
			// 如果是 Firing 那么将 EndsAt 设置为 nil
			alerts[i].EndsAt = nil

			// 校验静默
			isSilenced, silenceID := receiver.isSilenced(alerts[i], activeSilences)
			if isSilenced {
				alerts[i].IsSilenced = true
				alerts[i].SilenceID = silenceID
				silencedAlertMap[key] = alerts[i]
				zap.L().Info("告警被静默", zap.String("fingerprint", alerts[i].Fingerprint), zap.Int("silenceID", silenceID))
				// 被静默的告警不进入 firingAlertArry，不触发发送
				continue
			}

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
func (receiver *alertsService) saveAlerts(ctx context.Context, notifyReq *types.NotifyReq, sendResult *types.NotifySendResult) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			zap.L().Error("saveAlerts panic recovered",
				zap.Any("panic", r),
				zap.String("stack", string(stack)),
			)
		}
	}()

	log.WithRequestID(ctx).Debug("告警数据开始持久化")

	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var (
		allCreateSendRecords []*model.AlertSendRecord
		allUpdateSendRecords []*model.AlertSendRecord
		allUpdateAlerts      []*model.AlertHistory
	)

	var sharedAggRecord []*model.AlertSendRecord
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
		sharedAggRecord = append(sharedAggRecord, res.sharedAggRecord)
	}

	// --- 单独处理静默告警 ---
	silenceCreate, silenceUpdate := receiver.processSilencedAlerts(notifyReq)
	allUpdateAlerts = append(allUpdateAlerts, silenceUpdate...)

	log.WithRequestID(ctx).Debug("告警记录处理完成, 开始批量持久化")
	// 批量创建带有发送流水的告警 (Firing/Resolved)
	if len(allCreateSendRecords) > 0 {
		zap.L().Debug("批量创建告警历史记录")
		if err := aSend.WithContext(ctx).Create(allCreateSendRecords...); err != nil {
			log.WithRequestID(ctx).Error("批量创建告警历史记录失败", zap.Error(err))
		}
	}

	// 批量创建静默告警 (只有 History)
	if len(silenceCreate) > 0 {
		zap.L().Debug("批量创建静默告警历史")
		if err := aHistory.WithContext(ctx).Create(silenceCreate...); err != nil {
			log.WithRequestID(ctx).Error("批量创建静默告警历史失败", zap.Error(err))
		}
	}

	// 更新发送记录 (ErrorMessage 等)
	if len(allUpdateSendRecords) > 0 {
		zap.L().Debug("更新告警发送记录")
		for _, updateSendRecord := range allUpdateSendRecords {
			upObj := model.AlertSendRecord{
				ErrorMessage: updateSendRecord.ErrorMessage,
			}
			if _, err := aSend.WithContext(timeoutCtx).Where(aSend.ID.Eq(updateSendRecord.ID)).Updates(upObj); err != nil {
				log.WithRequestID(ctx).Error("批量更新告警发送记录失败", zap.Error(err))
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
			if _, err := aHistory.WithContext(timeoutCtx).Where(aHistory.ID.Eq(updateAlert.ID)).Updates(upMap); err != nil {
				log.WithRequestID(ctx).Error("批量更新告警历史记录失败", zap.Error(err))
				continue
			}
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

// CleanDuplicateFiringAlertsTask 定时清理任务：处理相同指纹但有多个 firing 状态的记录
func (receiver *alertsService) CleanDuplicateFiringAlertsTask() {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			zap.L().Error("cleanDuplicateFiringAlertsTask panic recovered",
				zap.Any("panic", r),
				zap.String("stack", string(stack)), // 这行会告诉你具体是代码哪一行崩了
			)
		}
	}()
	lockDuration := 5 * time.Minute
	ctx, cancle := context.WithTimeout(context.TODO(), 2*time.Minute)
	defer cancle()

	al := store.AlertHistory.WithContext(ctx)

	ok, err := receiver.cache.SetNX(ctx, store.LockType, constant.AlertCleanCacheLockKey, time.Now().Unix(), lockDuration)
	if err != nil {
		zap.L().Error("[定时任务] Redis 锁异常", zap.Error(err))
		return
	}

	// 没抢到锁，说明其他副本正在执行，直接退出
	if !ok {
		zap.L().Debug("[定时任务] 任务正在其他节点运行，本次跳过")
		return
	}

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

				zap.L().Warn("[定时任务] 发现重复 Firing 记录",
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

func (receiver *alertsService) isSilenced(alert *types.Alert, silences []*model.AlertSilence) (bool, int) {
	if len(silences) == 0 {
		return false, 0
	}

	alertLabels := alert.Labels

	for _, silence := range silences {
		var matchers []model.Matcher
		if err := json.Unmarshal(silence.Matchers, &matchers); err != nil {
			continue
		}
		// 时间窗口判断 (告警开始时间必须在静默期内)
		if alert.StartsAt.Before(silence.StartsAt) || alert.StartsAt.After(silence.EndsAt) {
			continue
		}

		allMatch := true
		for _, m := range matchers {
			val, ok := alertLabels[m.Name]
			if !ok {
				allMatch = false
				break
			}

			switch m.Type {
			case "=":
				if val != m.Value {
					allMatch = false
				}
			case "!=":
				if val == m.Value {
					allMatch = false
				}
			case "=~":
				// 注意：在生产环境下，建议提前在外部 Compile 好正则，此处直接使用对象
				matched, _ := regexp.MatchString("^("+m.Value+")$", val)
				if !matched {
					allMatch = false
				}
			case "!~":
				matched, _ := regexp.MatchString("^("+m.Value+")$", val)
				if matched {
					allMatch = false
				}
			default:
				allMatch = false
			}

			if !allMatch {
				break
			}
		}

		// 如果匹配器全部匹配，则返回成功及其静默规则 ID
		if allMatch {
			return true, silence.ID
		}
	}
	return false, 0
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
			storeHistory.AlertSilenceID = alert.SilenceID // 记录是哪个规则静默的
			updateAlerts = append(updateAlerts, storeHistory)
		} else {
			// 新告警即被静默
			alertHistory, err := types.ConvertToModel(receiver.tenantKey, alert, notifyReq.AlertChannel.ID)
			if err != nil {
				zap.L().Error("转换静默告警模型失败", zap.Error(err))
				continue
			}

			alertHistory.IsSilenced = true
			alertHistory.AlertSilenceID = alert.SilenceID
			alertHistory.AlertSendRecordID = nil
			createAlerts = append(createAlerts, alertHistory)
		}
	}
	return
}
