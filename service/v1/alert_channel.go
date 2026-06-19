package v1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"go.uber.org/zap"

	"github.com/alert666/api-server/base/constant"
	"github.com/alert666/api-server/base/log"
	"github.com/alert666/api-server/base/helper"
	"github.com/alert666/api-server/base/types"
	"github.com/alert666/api-server/model"
	"github.com/alert666/api-server/store"
	"gorm.io/gorm"
)

type AlertChannelServicer interface {
	CreateAlerChannel(ctx context.Context, req *types.AlertChannelCreateRequest) error
	UpdateChannel(ctx context.Context, req *types.AlertChannelUpdateRequest) error
	DeleteChannel(ctx context.Context, req *types.IDRequest) error
	QueryChannel(ctx context.Context, req *types.IDRequest) (*model.AlertChannel, error)
	ListChannel(ctx context.Context, req *types.AlertChannelListRequest) (*types.AlertChannelListResponse, error)
}

type alertChannelService struct {
	cache store.CacheStorer
}

func NewChannelServicer(cache store.CacheStorer) AlertChannelServicer {
	return &alertChannelService{
		cache: cache,
	}
}

func (receiver *alertChannelService) CreateAlerChannel(ctx context.Context, req *types.AlertChannelCreateRequest) error {
	log.WithRequestID(ctx).Debug("CreateAlerChannel", zap.String("name", req.Name), zap.String("type", req.Type))
	_, err := aChannelStore.WithContext(ctx).Where(aChannelStore.Name.Eq(req.Name)).First()
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		if err := helper.VerificationAlertConfig(req.Name, model.ChannelType(req.Type), req.Config); err != nil {
			return err
		}

		c, err := json.Marshal(req.Config)
		if err != nil {
			return err
		}

		obj := &model.AlertChannel{
			Name:              req.Name,
			Type:              model.ChannelType(req.Type),
			Status:            req.Status,
			AggregationStatus: req.AggregationStatus,
			Config:            c,
			Description:       req.Description,
		}

		if err := aChannelStore.WithContext(ctx).Create(obj); err != nil {
			return err
		}
		if err := receiver.cache.SetObject(ctx, store.AlertChannelType, obj.ID, obj, store.NeverExpires); err != nil {
		log.WithRequestID(ctx).Error("cache AlertChannel failed", zap.Int("id", obj.ID), zap.Error(err))
		}
		if obj.Type == model.ChannelTypeFeishuApp {
			config, err := obj.GetFeishuAppConfig()
			if err == nil {
				publish := fmt.Sprintf("%s:%s:%s", obj.Name, config.AppID, config.AppSecret)
				if err := receiver.cache.Publish(ctx, constant.AlertChannelTopicUpdate, publish); err != nil {
			log.WithRequestID(ctx).Error("publish channel create event failed", zap.Error(err))
				}
			}
		}
		return nil
	}
	return fmt.Errorf("alertChannel already exists, create failed")
}

func (receiver *alertChannelService) UpdateChannel(ctx context.Context, req *types.AlertChannelUpdateRequest) error {
	log.WithRequestID(ctx).Debug("UpdateChannel", zap.Int("id", int(req.ID)))
	sql := aChannelStore.WithContext(ctx)

	// 1. 加载旧数据
	acObj, err := sql.Where(aChannelStore.ID.Eq(int(req.ID))).First()
	if err != nil {
		return err
	}

	acObj.Type = model.ChannelType(req.Type)
	acObj.Status = req.Status
	acObj.AggregationStatus = req.AggregationStatus

	if err := helper.VerificationAlertConfig(acObj.Name, model.ChannelType(req.Type), req.Config); err != nil {
		return err
	}

	c, err := json.Marshal(req.Config)
	if err != nil {
		return err
	}
	acObj.Config = c
	acObj.Description = req.Description

	// redis publish 事件处理...
	var id, secret string
	if acObj.Type == model.ChannelTypeFeishuApp {
		config, err := acObj.GetFeishuAppConfig()
		if err != nil {
			return err
		}
		id = config.AppID
		secret = config.AppSecret
	}
	publish := fmt.Sprintf("%s:%s:%s", acObj.Name, id, secret)

	return store.Q.Transaction(func(tx *store.Query) error {
		// 3. 保存 Channel 记录
		if err := tx.AlertChannel.WithContext(ctx).Save(acObj); err != nil {
			return err
		}

		// 4. 清理旧缓存
		if err := receiver.cache.DelKey(ctx, store.AlertChannelType, acObj.ID); err != nil {
			return err
		}

		if *acObj.Status == model.StatusEnabled {
			// 6. 存入缓存的是带有最新 AlertTemplate 实体的 acObj
			if err := receiver.cache.SetObject(ctx, store.AlertChannelType, acObj.ID, acObj, store.NeverExpires); err != nil {
				return err
			}
			return receiver.cache.Publish(ctx, constant.AlertChannelTopicUpdate, publish)
		}
		return nil
	})
}

func (receiver *alertChannelService) DeleteChannel(ctx context.Context, req *types.IDRequest) error {
	log.WithRequestID(ctx).Debug("DeleteChannel", zap.Int("id", int(req.ID)))
	sql := aChannelStore.WithContext(ctx)

	acObj, err := sql.Where(aChannelStore.ID.Eq(int(req.ID))).First()
	if err != nil {
		return err
	}

	// 检查是否有告警模板绑定了该渠道，绑定了则不允许删除
	templates, err := aTemlpateStore.WithContext(ctx).Where(aTemlpateStore.AlertChannelID.Eq(acObj.ID)).Find()
	if err != nil {
		return err
	}
	if len(templates) > 0 {
		names := make([]string, 0, len(templates))
		for _, t := range templates {
			names = append(names, t.Name)
		}
		return fmt.Errorf("告警渠道 [%s] 已被告警模板 [%v] 绑定, 无法删除", acObj.Name, names)
	}

	var id, secret string
	switch acObj.Type {
	case model.ChannelTypeFeishuApp:
		config, err := acObj.GetFeishuAppConfig()
		if err != nil {
			return err
		}
		id = config.AppID
		secret = config.AppSecret
	}
	publish := fmt.Sprintf("%s:%s:%s", acObj.Name, id, secret)
	return store.Q.Transaction(func(tx *store.Query) error {
		_, err := tx.AlertChannel.WithContext(ctx).Where(aChannelStore.ID.Eq(acObj.ID)).Delete(acObj)
		if err != nil {
			return err
		}
		if err := receiver.cache.DelKey(ctx, store.AlertChannelType, acObj.ID); err != nil {
			return err
		}
		if acObj.Type != model.ChannelTypeEmail {
			return receiver.cache.Publish(ctx, constant.AlertChannelTopicDelete, publish)
		}
		return nil
	})
}

func (receiver *alertChannelService) QueryChannel(ctx context.Context, req *types.IDRequest) (*model.AlertChannel, error) {
	var cached model.AlertChannel
	found, err := receiver.cache.GetObject(ctx, store.AlertChannelType, int(req.ID), &cached)
	if err == nil && found {
		return &cached, nil
	}
	obj, err := aChannelStore.WithContext(ctx).Where(aChannelStore.ID.Eq(int(req.ID))).First()
	if err != nil {
		return nil, err
	}
	if err := receiver.cache.SetObject(ctx, store.AlertChannelType, obj.ID, obj, store.NeverExpires); err != nil {
		log.WithRequestID(ctx).Error("cache AlertChannel failed", zap.Int("id", obj.ID), zap.Error(err))
	}
	return obj, nil
}

func (receiver *alertChannelService) ListChannel(ctx context.Context, req *types.AlertChannelListRequest) (*types.AlertChannelListResponse, error) {
	var (
		alertChannels []*model.AlertChannel
		total         int64
		sql           = aChannelStore.WithContext(ctx)
		err           error
	)

	if req.Name != "" {
		sql = sql.Where(aChannelStore.Name.Like(req.Name + "%"))
	}

	if req.Type != "" {
		sql = sql.Where(aChannelStore.Type.Eq(req.Type))
	}

	if total, err = sql.Count(); err != nil {
		return nil, err
	}

	if req.Sort != "" && req.Direction != "" {
		sort, ok := aChannelStore.GetFieldByName(req.Sort)
		if !ok {
			return nil, fmt.Errorf("invalid sort field: %s", req.Sort)
		}
		sql = sql.Order(helper.Sort(sort, req.Direction))
	} else {
		sql = sql.Order(aChannelStore.CreatedAt.Desc())
	}

	if req.PageSize == 0 || req.Page == 0 {
		return nil, fmt.Errorf("pageSize 和 page 不能为0")
	}
	if alertChannels, err = sql.Limit(req.PageSize).Offset((req.Page - 1) * req.PageSize).Find(); err != nil {
		return nil, err
	}

	return types.NewAlertChannelListResponse(alertChannels, total, req.PageSize, req.Page), nil
}
