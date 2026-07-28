package v1

import (
	"context"
	"encoding/json"
	"regexp"
	"slices"
	"time"

	"github.com/alert666/api-server/base/config"
	"github.com/alert666/api-server/base/constant"
	"github.com/alert666/api-server/base/helper"
	"github.com/alert666/api-server/base/log"
	"github.com/alert666/api-server/base/types"
	"github.com/alert666/api-server/model"
	"go.uber.org/zap"
)

type IDCMetricser interface {
	// 获取指定时间范围的数据
	GetIDCMetrics(context.Context, *types.GetIDCMetricsReq) (*types.GetIDCMetricsRes, error)
	// 根据条件查询指定的 metrics
	QueryIDCMetrics(context.Context, *types.QueryIDCMetricsReq) (*types.QueryIDCMetricsRes, error)
	QueryImagePullDuration(context.Context, *types.QueryImagePullDurationReq) (*types.QueryImagePullDurationRes, error)
}

type idcMetrics struct {
}

func NewIDCMetrics() IDCMetricser {
	return &idcMetrics{}
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
		if err := json.Unmarshal(object.Labels, &labels); err != nil {
			log.WithRequestID(ctx).Error("序列化 labels 失败", zap.Int64("id", int64(object.ID)), zap.Any("labels", object.Labels), zap.Error(err))
			continue
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

		if err := json.Unmarshal(object.Labels, &labels); err != nil {
			return err
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

	objs, err := k8sEvent.WithContext(ctx).Where(
		k8sEvent.Cluster.Eq(cluster),
		k8sEvent.Reason.Eq(constant.K8SEventPulledReason),
		k8sEvent.LastSeen.Gte(st),
		k8sEvent.LastSeen.Lte(et),
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
