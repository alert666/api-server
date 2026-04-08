package v1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/alert666/api-server/base/constant"
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
	_, err := aChannel.WithContext(ctx).Unscoped().Where(aChannel.Name.Eq(req.Name)).First()
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

		return aChannel.WithContext(ctx).Create(obj)
	}
	return fmt.Errorf("alertChannel 已经存在, 创建失败")
}

func (receiver *alertChannelService) UpdateChannel(ctx context.Context, req *types.AlertChannelUpdateRequest) error {
	sql := aChannel.WithContext(ctx)

	acObj, err := sql.Preload(aChannel.AlertTemplate).Where(aChannel.ID.Eq(int(req.ID))).First()
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
	acObj.AlertTemplateID = req.TemplateID

	return store.Q.Transaction(func(tx *store.Query) error {
		if err := tx.AlertChannel.WithContext(ctx).Save(acObj); err != nil {
			return err
		}

		if err := receiver.cache.DelKey(ctx, store.AlertType, acObj.Name); err != nil {
			return err
		}

		if *acObj.Status == model.StatusEnabled {
			if acObj.AlertTemplate == nil {
				template, err := tx.AlertTemplate.WithContext(ctx).Where(aTemlpate.ID.Eq(acObj.AlertTemplateID)).First()
				if err != nil {
					return err
				}
				acObj.AlertTemplate = template
			}

			if err := receiver.cache.SetObject(ctx, store.AlertType, acObj.Name, acObj, store.NeverExpires); err != nil {
				return err
			}
			return receiver.cache.Publish(ctx, constant.AlertChannelTopicUpdate, acObj.Name)
		}
		return nil
	})
}

func (receiver *alertChannelService) DeleteChannel(ctx context.Context, req *types.IDRequest) error {
	sql := aChannel.WithContext(ctx)

	acObj, err := sql.Where(aChannel.ID.Eq(int(req.ID))).First()
	if err != nil {
		return err
	}

	return store.Q.Transaction(func(tx *store.Query) error {
		_, err := tx.AlertChannel.WithContext(ctx).Unscoped().Where(aChannel.ID.Eq(acObj.ID)).Delete(acObj)
		if err != nil {
			return err
		}
		if err := receiver.cache.DelKey(ctx, store.AlertType, acObj.Name); err != nil {
			return err
		}
		return receiver.cache.Publish(ctx, constant.AlertChannelTopicDelete, acObj.Name)
	})
}

func (receiver *alertChannelService) QueryChannel(ctx context.Context, req *types.IDRequest) (*model.AlertChannel, error) {
	obj, err := aChannel.WithContext(ctx).Where(aChannel.ID.Eq(int(req.ID))).First()
	if err != nil {
		return nil, err
	}

	return obj, nil
}

func (receiver *alertChannelService) ListChannel(ctx context.Context, req *types.AlertChannelListRequest) (*types.AlertChannelListResponse, error) {
	var (
		alertChannels []*model.AlertChannel
		total         int64
		sql           = aChannel.WithContext(ctx)
		err           error
	)

	if req.Name != "" {
		sql = sql.Where(aChannel.Name.Like("%" + req.Name + "%"))
	} else if req.Type != "" {
		sql.Where(aChannel.Type.Like("%" + req.Type + "%"))
	}

	if total, err = sql.Count(); err != nil {
		return nil, err
	}

	if req.Sort != "" && req.Direction != "" {
		sort, ok := aChannel.GetFieldByName(req.Sort)
		if !ok {
			return nil, fmt.Errorf("invalid sort field: %s", req.Sort)
		}
		sql = sql.Order(helper.Sort(sort, req.Direction))
	}

	if req.PageSize == 0 || req.Page == 0 {
		return nil, fmt.Errorf("pageSize 和 page 不能为0")
	}
	if alertChannels, err = sql.Limit(req.PageSize).Offset((req.Page - 1) * req.PageSize).Find(); err != nil {
		return nil, err
	}

	return types.NewAlertChannelListResponse(alertChannels, total, req.PageSize, req.Page), nil
}
