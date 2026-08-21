package v1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/alert666/api-server/base/config"
	"github.com/alert666/api-server/base/constant"
	"github.com/alert666/api-server/base/helper"
	"github.com/alert666/api-server/base/log"
	"github.com/alert666/api-server/base/types"
	"github.com/alert666/api-server/model"
	"github.com/alert666/api-server/store"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type IDCMetricser interface {
	// 获取指定时间范围的数据
	GetIDCMetrics(context.Context, *types.GetIDCMetricsReq) (*types.GetIDCMetricsRes, error)
	// 根据条件查询指定的 metrics
	QueryIDCMetrics(context.Context, *types.QueryIDCMetricsReq) (*types.QueryIDCMetricsRes, error)
	QueryImagePullDuration(context.Context, *types.QueryImagePullDurationReq) (*types.QueryImagePullDurationRes, error)
	// IDC 机房心跳检测
	IDCHeartbeat(context.Context, *types.IDCHeartbeatReq) error
	// 删除过时的心跳记录
	DeleteIDCHeartbeat(context.Context, *types.DeleteIDCHeartbeatReq) error
}

type idcMetrics struct {
	cacheImpl store.CacheStorer
	alertImpl AlertsServicer
}

func NewIDCMetrics(cacheImpl store.CacheStorer) IDCMetricser {
	return &idcMetrics{
		cacheImpl: cacheImpl,
	}
}

func (idc *idcMetrics) GetIDCMetrics(ctx context.Context, req *types.GetIDCMetricsReq) (*types.GetIDCMetricsRes, error) {
	cluster, err := helper.GetTenant(ctx)
	if err != nil {
		return nil, err
	}
	st := time.Unix(req.StartTimestamp, 0)
	et := time.Unix(req.EndTimestamp, 0)
	alertHistorys := make([]*model.AlertHistory, 0, 30)

	switch req.AlertName {
	case constant.GPUCardLossName:
		for _, v := range constant.GPUCardLossValues {
			_alertHistorys, err := getIDCMetricser(ctx, v, cluster, st, et)
			if err != nil {
				return nil, err
			}
			alertHistorys = append(alertHistorys, _alertHistorys...)
		}
	default:
		_alertHistorys, err := getIDCMetricser(ctx, req.AlertName, cluster, st, et)
		if err != nil {
			return nil, err
		}
		alertHistorys = append(alertHistorys, _alertHistorys...)
	}

	res := types.NewGetIDCMetricsRes()
	res.Cluster = cluster
	res.StartTimestamp = req.StartTimestamp
	res.EndTimestamp = req.EndTimestamp

	for _, object := range alertHistorys {
		if object == nil {
			return nil, nil
		}

		var labels map[string]string
		if object.Labels != nil {
			if err := json.Unmarshal(object.Labels, &labels); err != nil {
				log.WithRequestID(ctx).Error("序列化 labels 失败", zap.Int64("id", int64(object.ID)), zap.Any("labels", object.Labels), zap.Error(err))
				continue
			}
		}

		var (
			alertEndTimestamp *int64
			node              string
		)
		if object.EndsAt != nil {
			alertEndTimestamp = new(object.EndsAt.Unix())
		}

		if _, ok := labels["node"]; ok {
			node = labels["node"]
		} else {
			node = labels["Hostname"]
		}

		res.IDCMetrics = append(res.IDCMetrics,
			types.IDCMetrics{
				Node:                node,
				IP:                  labels["internal_ip"],
				AlertStartTimestamp: object.StartsAt.Unix(),
				AlertEndTimestamp:   alertEndTimestamp,
			},
		)
	}
	res.AlertName = req.AlertName
	return res, nil
}

func getIDCMetricser(ctx context.Context, alertName, cluster string, st, et time.Time) ([]*model.AlertHistory, error) {
	objects, err := aHistoryStore.WithContext(ctx).
		Select(
			aHistoryStore.ID,
			aHistoryStore.Cluster,
			aHistoryStore.Alertname,
			aHistoryStore.StartsAt,
			aHistoryStore.EndsAt,
			aHistoryStore.Labels,
		).
		Where(
			aHistoryStore.Cluster.Eq(cluster),
			aHistoryStore.Alertname.Eq(alertName),
			aHistoryStore.StartsAt.Gte(st),
			aHistoryStore.StartsAt.Lte(et),
		).Find()
	if err != nil {
		return nil, err
	}
	return objects, nil
}

func (idc *idcMetrics) QueryIDCMetrics(ctx context.Context, req *types.QueryIDCMetricsReq) (*types.QueryIDCMetricsRes, error) {
	cluster, err := helper.GetTenant(ctx)
	if err != nil {
		return nil, err
	}

	res := types.NewQueryIDCMetricsRes()
	res.Cluster = cluster

	for _, v := range req.QueryIDCMetrics {
		switch v.AlertName {
		case constant.GPUCardLossName:
			for _, alertname := range constant.GPUCardLossValues {
				v.AlertName = alertname
				if err = queryIDCmetrics(ctx, cluster, v, res); err != nil {
					return nil, err
				}
			}
		default:
			if err = queryIDCmetrics(ctx, cluster, v, res); err != nil {
				return nil, err
			}
		}
	}
	return res, nil
}

func queryIDCmetrics(ctx context.Context, cluster string, req types.QueryIDCMetrics, res *types.QueryIDCMetricsRes) error {
	st := time.Unix(req.StartTimestamp, 0).Truncate(time.Millisecond)
	objects, err := aHistoryStore.WithContext(ctx).
		Select(
			aHistoryStore.Cluster,
			aHistoryStore.Alertname,
			aHistoryStore.StartsAt,
			aHistoryStore.EndsAt,
			aHistoryStore.Labels,
		).
		Where(
			aHistoryStore.Cluster.Eq(cluster),
			aHistoryStore.Alertname.Eq(req.AlertName),
			aHistoryStore.StartsAt.Gte(st),
			aHistoryStore.StartsAt.Lt(st.Add(time.Second)),
		).Find()
	if err != nil {
		return err
	}

	if len(objects) == 0 {
		return nil
	}

	for _, object := range objects {
		var (
			node, ip          string
			ok                bool
			labels            map[string]string
			alertEndTimestamp *int64
		)

		if object.Labels != nil {
			if err := json.Unmarshal(object.Labels, &labels); err != nil {
				return fmt.Errorf("获取对象 labels 失败, %w", err)
			}
		}

		if node, ok = labels["node"]; !ok {
			if hostname, ok := labels["Hostname"]; ok {
				if hostname != req.Node {
					continue
				}
			}
		} else {
			if node != req.Node {
				continue
			}
		}

		if ip, ok = labels["internal_ip"]; ok {
			if ip != req.IP {
				continue
			}
		}

		if object.EndsAt != nil {
			alertEndTimestamp = new(object.EndsAt.Unix())
		}
		if slices.Contains(constant.GPUCardLossValues, object.Alertname) {
			object.Alertname = constant.GPUCardLossName
		}

		res.QueryIDCMetricsres = append(res.QueryIDCMetricsres, types.QueryIDCMetricsres{
			AlertName: object.Alertname,
			IDCMetrics: &types.IDCMetrics{
				Node:                node,
				IP:                  ip,
				AlertStartTimestamp: object.StartsAt.Unix(),
				AlertEndTimestamp:   alertEndTimestamp,
			},
		})
	}
	return nil
}

func (idc *idcMetrics) QueryImagePullDuration(ctx context.Context, req *types.QueryImagePullDurationReq) (*types.QueryImagePullDurationRes, error) {
	cluster, err := helper.GetTenant(ctx)
	if err != nil {
		return nil, err
	}
	st := time.Unix(req.StartTimestamp, 0)
	et := time.Unix(req.EndTimestamp, 0)

	objs, err := k8sEventStore.WithContext(ctx).Where(
		k8sEventStore.Cluster.Eq(cluster),
		k8sEventStore.Reason.Eq(constant.K8SEventPulledReason),
		k8sEventStore.LastSeen.Gte(st),
		k8sEventStore.LastSeen.Lte(et),
	).Find()
	if err != nil {
		return nil, err
	}

	var (
		results         = make([]*types.PulledImageEvent, 0, len(objs))
		durationSeconds uint64
		sizeByte        uint64
	)
	for _, obj := range objs {
		pulledImageEvent, err := helper.ParseImagePullEvent(obj.Message)
		if err != nil {
			return nil, err
		}
		if pulledImageEvent != nil {
			results = append(results, pulledImageEvent)
			durationSeconds += pulledImageEvent.DurationSeconds
			sizeByte += pulledImageEvent.SizeByte
		}
	}

	return &types.QueryImagePullDurationRes{
		PulledImageEvents: results,
		DurationSeconds:   durationSeconds,
		SizeByte:          sizeByte,
		StartTimestamp:    req.StartTimestamp,
		EndTimestamp:      req.EndTimestamp,
	}, nil
}

var kubernetesEventFieldGetter = map[string]func(*types.KubernetesEventReceiveReq) string{
	"metadataName": func(e *types.KubernetesEventReceiveReq) string {
		return e.MetadataName
	},
	"apiVersion": func(e *types.KubernetesEventReceiveReq) string {
		return e.ApiVersion
	},
	"firstTimestamp": func(e *types.KubernetesEventReceiveReq) string {
		return e.FirstTimestamp
	},
	"lastTimestamp": func(e *types.KubernetesEventReceiveReq) string {
		return e.LastTimestamp
	},
	"eventTime": func(e *types.KubernetesEventReceiveReq) string {
		return e.EventTime
	},
	"kind": func(e *types.KubernetesEventReceiveReq) string {
		return e.Kind
	},
	"message": func(e *types.KubernetesEventReceiveReq) string {
		return e.Message
	},

	"name": func(e *types.KubernetesEventReceiveReq) string {
		return e.Name
	},
	"namespace": func(e *types.KubernetesEventReceiveReq) string {
		return e.Namespace
	},
	"reason": func(e *types.KubernetesEventReceiveReq) string {
		return e.Reason
	},
	"reportingComponent": func(e *types.KubernetesEventReceiveReq) string {
		return e.ReportingComponent
	},
	"reportingInstance": func(e *types.KubernetesEventReceiveReq) string {
		return e.ReportingInstance
	},
	"type": func(e *types.KubernetesEventReceiveReq) string {
		return e.Type
	},
	"firstTimestampTime": func(e *types.KubernetesEventReceiveReq) string {
		if e.FirstTimestampTime == nil {
			return ""
		}
		return e.FirstTimestampTime.Format(time.RFC3339)
	},
	"lastTimestampTime": func(e *types.KubernetesEventReceiveReq) string {
		if e.LastTimestampTime == nil {
			return ""
		}
		return e.LastTimestampTime.Format(time.RFC3339)
	},
	"eventTimeTime": func(e *types.KubernetesEventReceiveReq) string {
		if e.EventTimeTime == nil {
			return ""
		}
		return e.EventTimeTime.Format(time.RFC3339)
	},
}

func matchKubernetesEvent(req *types.KubernetesEventReceiveReq, config *config.KubernetesEventsConfig) bool {
	for field, regex := range config.Exclude {
		getter, ok := kubernetesEventFieldGetter[field]
		if !ok {
			continue
		}
		value := getter(req)

		matched, err := regexp.MatchString(regex, value)
		if err != nil {
			continue
		}
		if matched {
			return true
		}
	}
	return false
}

// IDCHeartbeat idc 机房心跳
func (idc *idcMetrics) IDCHeartbeat(ctx context.Context, req *types.IDCHeartbeatReq) error {
	cluster, err := helper.GetTenant(ctx)
	if err != nil {
		return err
	}

	return store.Q.Transaction(func(tx *store.Query) error {
		objects, err := tx.IDCHeartbeat.WithContext(ctx).Where(
			tx.IDCHeartbeat.Cluster.Eq(cluster),
			tx.IDCHeartbeat.Node.Eq(req.Node),
		).Find()
		if err != nil {
			return err
		}

		deleteObj := make([]*model.IDCHeartbeat, 0, len(objects))
		keepObj := make([]*model.IDCHeartbeat, 0, len(objects))
		for _, object := range objects {
			if req.IP == object.IP {
				keepObj = append(keepObj, object)
			} else {
				deleteObj = append(deleteObj, object)
			}
		}

		if len(deleteObj) > 0 {
			if _, err := tx.IDCHeartbeat.WithContext(ctx).Delete(deleteObj...); err != nil {
				return fmt.Errorf("删除老数据失败, %w", err)
			}
		}

		// 没有的话需要新建记录
		if len(keepObj) == 0 {
			modelObj := &model.IDCHeartbeat{
				Cluster:            cluster,
				Node:               req.Node,
				IP:                 req.IP,
				HeartbeatTimestamp: req.HeartbeatTimestamp,
			}
			if err := tx.IDCHeartbeat.WithContext(ctx).Create(modelObj); err != nil {
				return fmt.Errorf("创建 idcHeartbeat 失败, %w", err)
			}
			return nil
		}

		latestObj := keepObj[0]
		for _, obj := range keepObj[1:] {
			if obj.CreatedAt.After(latestObj.CreatedAt) {
				latestObj = obj
			}
		}

		if _, err := tx.IDCHeartbeat.WithContext(ctx).
			Where(tx.IDCHeartbeat.ID.Eq(latestObj.ID)).
			Update(tx.IDCHeartbeat.HeartbeatTimestamp, req.HeartbeatTimestamp); err != nil {
			return fmt.Errorf("更新 idcHeartbeat 失败, %w", err)
		}
		return nil
	})
}

// CronJobIDCMetricser idcmetrics 定时任务
type CronJobIDCMetricser interface {
	CronJobIDCHeartbeat()
	CronJobIDCResolvedHeartbeat()
}

func NewIDCHeartbeat(cacheImpl store.CacheStorer, alertImpl AlertsServicer) CronJobIDCMetricser {
	return &idcMetrics{
		cacheImpl: cacheImpl,
		alertImpl: alertImpl,
	}
}

func (idc *idcMetrics) CronJobIDCResolvedHeartbeat() {
	start := time.Now()
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			zap.L().Error("CronJobIDCResolvedHeartbeat panic recovered",
				zap.Any("panic", r),
				zap.String("stack", string(stack)),
			)
			return
		}
		elapsed := time.Since(start).Milliseconds()
		zap.L().Debug("CronJobIDCResolvedHeartbeat 执行结束",
			zap.Int64("duration_ms", elapsed),
		)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 58*time.Second)
	defer cancel()

	ok, err := idc.cacheImpl.SetNX(ctx, store.LockType, constant.CronJobResolvedIDCHeartbeatLockKey, time.Now().Unix(), 58*time.Second)
	if err != nil {
		zap.L().Error("[定时任务] CronJobIDCResolvedHeartbeat Redis 分布式锁异常", zap.Error(err))
		return
	}
	defer idc.cacheImpl.DelKey(ctx, store.LockType, constant.CronJobResolvedIDCHeartbeatLockKey)
	if !ok {
		zap.L().Debug("[定时任务] CronJobIDCResolvedHeartbeat 任务正在其他节点运行，本次跳过")
		return
	}

	var innerRes []*types.TenantOption
	_, err = idc.cacheImpl.GetObject(ctx, store.TenantType, constant.OptionsCacheKey, &innerRes)
	if err != nil {
		zap.L().Debug("[定时任务] CronJobIDCResolvedHeartbeat 获取租户失败，任务结束")
		return
	}

	var wg sync.WaitGroup
	for _, tenant := range innerRes {
		wg.Add(1)
		go func(tenant *types.TenantOption) {
			defer wg.Done()
			cronJobIDCResolvedHeartbeat(ctx, tenant, idc.alertImpl)
		}(tenant)
	}

	wg.Wait()
}

func cronJobIDCResolvedHeartbeat(ctx context.Context, t *types.TenantOption, alertImpl AlertsServicer) {
	alertHistoryObj, err := aHistoryStore.WithContext(ctx).Where(
		aHistoryStore.Cluster.Eq(t.Value),
		aHistoryStore.Alertname.Eq(constant.IDCHeartbeatAlertName),
		aHistoryStore.Status.Eq(constant.AlertStatusFiring),
	).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			zap.L().Debug("[定时任务] CronJobIDCResolvedHeartbeat 没有正在 firing 的告警, 定时任务退出")
			return
		}
		zap.L().Error("[定时任务] cronJobIDCResolvedHeartbeat 获取正在 firing 的告警失败", zap.String("cluster", t.Value), zap.Error(err))
		return
	}

	objects, err := idcStore.WithContext(ctx).Where(idcStore.Cluster.Eq(t.Value)).Find()
	if err != nil {
		zap.L().Error("[定时任务] cronJobIDCResolvedHeartbeat 获取正在 IDCHeartbeat 记录失败", zap.String("cluster", t.Value), zap.Error(err))
		return
	}

	var (
		firingNodeCount float64
		now             = time.Now()
	)
	for _, object := range objects {
		if now.Sub(object.UpdatedAt) >= constant.IDCHeartbeatThreshold {
			firingNodeCount++
		}
	}

	// 如果当前没有心跳的节点数大于阈值, 那么告警记录不做修改
	idcHeartbeatFiring := getIDCHeartbeatCout(len(objects))
	if len(objects) != 0 && firingNodeCount >= idcHeartbeatFiring {
		return
	}

	alertHistoryObj.Status = constant.AlertStatusResolved
	alertHistoryObj.EndsAt = new(now)
	if alertHistoryObj.Status == constant.AlertStatusResolved {
		sendAlert(ctx, alertImpl, t, alertHistoryObj)
	}

	// // 如果当前没有心跳的节点数小于阈值, 那么告警记录需要修改为恢复
	// _, err = aHistoryStore.WithContext(ctx).Where(aHistoryStore.ID.Eq(alertHistoryObj.ID)).UpdateColumnSimple(
	// 	aHistoryStore.Status.Value(constant.AlertStatusResolved),
	// 	aHistoryStore.EndsAt.Value(now),
	// )
	// if err != nil {
	// 	zap.L().Error("[定时任务] cronJobIDCResolvedHeartbeat 更新状态失败", zap.Any("alertHistory", alertHistoryObj), zap.Error(err))
	// 	return
	// }
}

// IDCHeartbeat idc 机房心跳
func (idc *idcMetrics) CronJobIDCHeartbeat() {
	start := time.Now()
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			zap.L().Error("CronJobIDCHeartbeat panic recovered",
				zap.Any("panic", r),
				zap.String("stack", string(stack)),
			)
			return
		}
		elapsed := time.Since(start).Milliseconds()
		zap.L().Debug("CronJobIDCHeartbeat 执行结束",
			zap.Int64("duration_ms", elapsed),
		)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 1000000*time.Second)
	defer cancel()

	ok, err := idc.cacheImpl.SetNX(ctx, store.LockType, constant.CronJobIDCHeartbeatLockKey, time.Now().Unix(), 58*time.Second)
	if err != nil {
		zap.L().Error("[定时任务] CronJobIDCHeartbeat Redis 分布式锁异常", zap.Error(err))
		return
	}
	defer idc.cacheImpl.DelKey(ctx, store.LockType, constant.CronJobIDCHeartbeatLockKey)

	if !ok {
		zap.L().Debug("[定时任务] CronJobIDCHeartbeat 任务正在其他节点运行，本次跳过")
		return
	}

	zap.L().Debug("[定时任务] CronJobIDCHeartbeat 成功获取锁，开始执行")
	var innerRes []*types.TenantOption
	_, err = idc.cacheImpl.GetObject(ctx, store.TenantType, constant.OptionsCacheKey, &innerRes)
	if err != nil {
		zap.L().Debug("[定时任务] CronJobIDCHeartbeat 获取租户失败，任务结束")
		return
	}

	var wg sync.WaitGroup
	for _, tenant := range innerRes {
		wg.Add(1)

		go func(t *types.TenantOption) {
			defer wg.Done()
			processIDCHeartbeat(ctx, idc.alertImpl, t)
		}(tenant)
	}

	// 等待子协程结束
	wg.Wait()
}

func processIDCHeartbeat(ctx context.Context, alertImpl AlertsServicer, tenant *types.TenantOption) {
	objects, err := idcStore.WithContext(ctx).Where(idcStore.Cluster.Eq(tenant.Value)).Find()
	if err != nil {
		zap.L().Error("[定时任务] CronJobIDCHeartbeat 查询 IDCHeartbeat 失败", zap.String("cluster", tenant.Value), zap.Error(err))
		return
	}

	var (
		now             = time.Now()
		firingNodeCount float64
		aHObj           *model.AlertHistory
	)

	for _, object := range objects {
		if now.Sub(object.UpdatedAt) >= constant.IDCHeartbeatThreshold {
			firingNodeCount++
		}
	}

	// 如果当前没有心跳的节点数小于阈值，那么不需要创建告警记录
	if len(objects) == 0 {
		return
	}
	idcHeartbeatFiring := getIDCHeartbeatCout(len(objects))
	if firingNodeCount < idcHeartbeatFiring {
		return
	}

	// 发送告警记录
	defer func() {
		if aHObj.SendCount == 1 {
			sendAlert(ctx, alertImpl, tenant, aHObj)
		}
	}()

	// 如果当前没有心跳的节点数大于阈值，那么需要创建告警记录
	// 查询当前节点是否有firing的告警
	aHObj, err = aHistoryStore.WithContext(ctx).Where(
		aHistoryStore.Cluster.Eq(tenant.Value),
		aHistoryStore.Alertname.Eq(constant.IDCHeartbeatAlertName),
		aHistoryStore.Status.Eq(constant.AlertStatusFiring),
	).First()
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			zap.L().Error("[定时任务] CronJobIDCHeartbeat 获取正在 firing 的告警失败",
				zap.String("cluster", tenant.Value),
				zap.Error(err),
			)
			return
		}

		annotations := make(map[string]any, 1)
		annotations["description"] = fmt.Sprintf("集群%s网络从内部探测外部不可用", tenant.Label)
		annotationsByte, err := json.Marshal(&annotations)
		if err != nil {
			zap.L().Error("[定时任务] CronJobIDCHeartbeat 创建告警 annotations 失败",
				zap.String("cluster", tenant.Value),
				zap.Any("annotations", annotations),
				zap.Error(err),
			)
			return
		}

		labels := make(map[string]string, 1)
		labels["alertname"] = constant.IDCHeartbeatAlertName
		labels["severity"] = "P0"
		labels["cluster"] = tenant.Value

		labelsByte, err := json.Marshal(&labels)
		if err != nil {
			zap.L().Error("[定时任务] CronJobIDCHeartbeat 创建告警 labels 失败",
				zap.String("cluster", tenant.Value),
				zap.Any("labels", labels),
				zap.Error(err),
			)
			return
		}

		ahModel := &model.AlertHistory{
			Cluster:         tenant.Value,
			Fingerprint:     uuid.NewString(),
			StartsAt:        now,
			Alertname:       constant.IDCHeartbeatAlertName,
			Status:          constant.AlertStatusFiring,
			Severity:        "P0",
			Labels:          labelsByte,
			Annotations:     annotationsByte,
			SendCount:       1,
			AlertTemplateID: 18,
		}

		if err := aHistoryStore.WithContext(ctx).Create(ahModel); err != nil {
			zap.L().Error("[定时任务] CronJobIDCHeartbeat 创建告警失败",
				zap.Any("alertHistory", ahModel),
				zap.Error(err),
			)
		}

		aHObj = ahModel
		return
	}

	sendcount := aHObj.SendCount + 1
	_, err = aHistoryStore.WithContext(ctx).Where(aHistoryStore.ID.Eq(aHObj.ID)).UpdateColumnSimple(
		aHistoryStore.SendCount.Value(sendcount),
	)
	if err != nil {
		zap.L().Error("[定时任务] CronJobIDCHeartbeat 更新正在 firing 的告警失败",
			zap.String("cluster", tenant.Value),
			zap.Any("alertHistory", aHObj),
			zap.Error(err),
		)
	}
}

func getIDCHeartbeatCout(totalNode int) float64 {
	return float64(totalNode) * constant.IDCHeartbeatFiringProportion
}

func (idc *idcMetrics) DeleteIDCHeartbeat(ctx context.Context, req *types.DeleteIDCHeartbeatReq) error {
	cluster, err := helper.GetTenant(ctx)
	if err != nil {
		return err
	}

	_, err = idcStore.WithContext(ctx).Where(
		idcStore.Cluster.Eq(cluster),
		idcStore.IP.Eq(req.IP),
	).Delete()
	if err != nil {
		return fmt.Errorf("删除过时记录失败, %w", err)
	}

	return nil
}

func sendAlert(ctx context.Context, alertImpl AlertsServicer, tenant *types.TenantOption, aHObj *model.AlertHistory) {
	templateNames, err := config.GetAlertEvaluateTemplateName()
	if err != nil {
		zap.L().Error("[定时任务] CronJobIDCHeartbeat, 获取告警模板名称失败",
			zap.String("tenant", tenant.Value),
			zap.String("alertName", constant.IDCHeartbeatAlertName),
			zap.Error(err),
		)
		return
	}

	// 获取对应的 templateName
	idcHeartbeatAlertName := strings.ToLower(constant.IDCHeartbeatAlertName)
	tn, ok := templateNames[idcHeartbeatAlertName]
	if !ok {
		zap.L().Error("[定时任务] CronJobIDCHeartbeat, 未找到对应的告警模板",
			zap.String("tenant", tenant.Value),
			zap.String("alertName", constant.IDCHeartbeatAlertName),
		)
		return
	}

	for _, templateName := range tn {
		annotations := aHObj.Annotations
		annotationsMap := make(map[string]string)
		if templateName == "zhongbao" {
			annotationsMap["description"] = `【集群性能通知】【网络影响】尊敬的客户，您好！<br>故障描述：平台监控到<font color='red'>**$labels.cluster**</font>网络出现间歇性抖动，可能影响您的业务访问体验，该影响平台持续关注中，若您有任何问题，请随时联系我们。`

			annotationsBytes, err := json.Marshal(&annotationsMap)
			if err != nil {
				zap.L().Error("[定时任务] CronJobIDCHeartbeat, 发送告警失败, 序列化 annotations 失败",
					zap.String("tenant", tenant.Value),
					zap.String("alertName", constant.IDCHeartbeatAlertName),
					zap.Any("data", aHObj),
					zap.Error(err),
				)
				continue
			}
			aHObj.Annotations = annotationsBytes
		}

		req, err := types.TransformationAlertHistoryToAlertReq(tenant.Value, []*model.AlertHistory{aHObj})
		if err != nil {
			zap.L().Error("[定时任务] CronJobIDCHeartbeat, 发送告警失败, 转换 AlertHistory 到 AlertReq 失败",
				zap.String("tenant", tenant.Value),
				zap.String("alertName", constant.IDCHeartbeatAlertName),
				zap.Any("data", req),
				zap.Error(err),
			)
			continue
		}

		err = json.Unmarshal(aHObj.Annotations, &annotationsMap)
		if err != nil {
			zap.L().Error("[定时任务] CronJobIDCHeartbeat, 发送告警失败, 解析 annotations 失败",
				zap.String("tenant", tenant.Value),
				zap.String("alertName", constant.IDCHeartbeatAlertName),
				zap.Any("data", req),
				zap.Error(err),
			)
			continue
		}

		req.TemplateName = templateName
		if err := alertImpl.SendAlert(ctx, req); err != nil {
			zap.L().Error("[定时任务] CronJobIDCHeartbeat, 发送告警失败",
				zap.String("tenant", tenant.Value),
				zap.String("alertName", constant.IDCHeartbeatAlertName),
				zap.Any("data", req),
				zap.Error(err),
			)
		}
		aHObj.Annotations = annotations
	}
}
