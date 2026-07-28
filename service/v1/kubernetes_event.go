package v1

import (
	"context"
	"fmt"
	"time"

	"github.com/alert666/api-server/base/config"
	"github.com/alert666/api-server/base/helper"
	"github.com/alert666/api-server/base/log"
	"github.com/alert666/api-server/base/types"
	"github.com/alert666/api-server/model"
	"go.uber.org/zap"
)

type KubernetesEventServicer interface {
	ReceiveEvent(context.Context, *types.KubernetesEventReceiveReq) error
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
		log.WithRequestID(ctx).Debug("命中过滤规则, 忽略事件", zap.Any("event", req))
		return nil
	}

	// 1. 检查是否存在该事件
	count, err := k8sEventStore.WithContext(ctx).Where(
		k8sEventStore.Cluster.Eq(cluster),
		k8sEventStore.Namespace.Eq(req.Namespace),
		k8sEventStore.MetadataName.Eq(req.MetadataName),
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
		_, err = k8sEventStore.WithContext(ctx).Where(
			k8sEventStore.Cluster.Eq(cluster),
			k8sEventStore.Namespace.Eq(req.Namespace),
			k8sEventStore.MetadataName.Eq(req.MetadataName),
		).UpdateSimple(
			k8sEventStore.Count_.Add(1),
			k8sEventStore.LastSeen.Value(lastSeen),
			k8sEventStore.Message.Value(req.Message),
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

	err = k8sEventStore.WithContext(ctx).Create(obj)
	if err != nil {
		return fmt.Errorf("创建事件失败: %w", err)
	}
	return nil
}
