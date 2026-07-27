package v1

import (
	"context"
	"encoding/json"
	"regexp"
	"time"

	"github.com/alert666/api-server/base/config"
	"github.com/alert666/api-server/base/constant"
	"github.com/alert666/api-server/base/helper"
	"github.com/alert666/api-server/base/log"
	"github.com/alert666/api-server/base/types"
	"github.com/alert666/api-server/model"
	"go.uber.org/zap"
	"gorm.io/gen/field"
)

type IDCMetricser interface {
	GetIDCMetricser(context.Context, *types.QeryIDCMetricsReq) (*types.QeryIDCMetricsRes, error)
	QueryImagePullDuration(context.Context, *types.QueryImagePullDurationReq) (*types.QueryImagePullDurationRes, error)
}

type idcMetrics struct {
}

func NewIDCMetrics() IDCMetricser {
	return &idcMetrics{}
}

func (idc *idcMetrics) GetIDCMetricser(ctx context.Context, req *types.QeryIDCMetricsReq) (*types.QeryIDCMetricsRes, error) {
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

	res := types.NewQeryIDCMetricsRes()
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
			field.Or(
				aHistoryStore.EndsAt.Lte(et),
				aHistoryStore.EndsAt.IsNull(),
			),
		).Find()
	if err != nil {
		return nil, err
	}
	return objects, nil
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
