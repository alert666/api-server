package v1

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/alert666/api-server/base/config"
	"github.com/alert666/api-server/base/constant"
	"github.com/alert666/api-server/base/helper"
	"github.com/alert666/api-server/base/log"
	"github.com/alert666/api-server/base/types"
	"github.com/alert666/api-server/model"
	"go.uber.org/zap"
)

type KubernetesEventServicer interface {
	ReceiveEvent(context.Context, *types.KubernetesEventReceiveReq) error
	QueryImagePullDuration(context.Context, *types.QueryImagePullDurationReq) (*types.QueryImagePullDurationRes, error)
}

type kubernetesEventService struct {
	kubernetesEventsConfig *config.KubernetesEventsConfig
}

func NewKubernetesEventServicer(kubernetesEventsConfig *config.KubernetesEventsConfig) KubernetesEventServicer {
	return &kubernetesEventService{
		kubernetesEventsConfig: kubernetesEventsConfig,
	}
}

func (ke *kubernetesEventService) ReceiveEvent(ctx context.Context, req *types.KubernetesEventReceiveReq) error {
	cluster, err := helper.GetTenant(ctx)
	if err != nil {
		return err
	}

	if ke.kubernetesEventsConfig.PrintReceivedData {
		log.WithRequestID(ctx).Info(
			"kubernetes 事件详情",
			zap.Any("event", req),
		)
	}

	exclude := matchKubernetesEvent(req, ke.kubernetesEventsConfig)
	if exclude {
		log.WithRequestID(ctx).Info("命中过滤规则, 忽略事件", zap.Any("event", req))
		return nil
	}

	// 1. 检查是否存在该事件
	count, err := k8sEvent.WithContext(ctx).Where(
		k8sEvent.Cluster.Eq(cluster),
		k8sEvent.Namespace.Eq(req.Namespace),
		k8sEvent.MetadataName.Eq(req.MetadataName),
	).Count()
	if err != nil {
		return fmt.Errorf("查询数据库失败: %w", err)
	}

	// 安全保护：防止在外部入参未传时间时，指针解引用 (*) 导致程序 Panic
	var firstSeen, lastSeen time.Time
	if req.FirstTimestampTime != nil {
		firstSeen = *req.FirstTimestampTime
	} else {
		firstSeen = time.Now()
	}
	if req.LastTimestampTime != nil {
		lastSeen = *req.LastTimestampTime
	} else {
		lastSeen = time.Now()
	}

	// 2. 存在则更新部分字段（count+1、LastSeen、Message）
	if count > 0 {
		_, err = k8sEvent.WithContext(ctx).Where(
			k8sEvent.Cluster.Eq(cluster),
			k8sEvent.Namespace.Eq(req.Namespace),
			k8sEvent.MetadataName.Eq(req.MetadataName),
		).UpdateSimple(
			k8sEvent.Count_.Add(1),
			k8sEvent.LastSeen.Value(lastSeen),
			k8sEvent.Message.Value(req.Message),
		)
		if err != nil {
			return fmt.Errorf("更新事件失败: %w", err)
		}
		return nil
	}

	obj := &model.KubernetesEvent{
		Cluster:            cluster,
		Namespace:          req.Namespace,
		MetadataName:       req.MetadataName,
		Reason:             req.Reason,
		Name:               req.Name,
		Kind:               req.Kind,
		LastSeen:           lastSeen,
		FirstSeen:          firstSeen,
		Type:               req.Type,
		Message:            req.Message,
		Count:              1,
		ReportingComponent: req.ReportingComponent,
		ReportingInstance:  req.ReportingInstance,
		ApiVersion:         req.ApiVersion,
	}

	err = k8sEvent.WithContext(ctx).Create(obj)
	if err != nil {
		return fmt.Errorf("创建事件失败: %w", err)
	}
	return nil
}

func (ke *kubernetesEventService) QueryImagePullDuration(ctx context.Context, req *types.QueryImagePullDurationReq) (*types.QueryImagePullDurationRes, error) {
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
