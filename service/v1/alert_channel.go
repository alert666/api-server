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
	_, err := aChannelStore.WithContext(ctx).Unscoped().Where(aChannelStore.Name.Eq(req.Name)).First()
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

		return aChannelStore.WithContext(ctx).Create(obj)
	}
	return fmt.Errorf("alertChannel 已经存在, 创建失败")
}

func (receiver *alertChannelService) UpdateChannel(ctx context.Context, req *types.AlertChannelUpdateRequest) error {
	sql := aChannelStore.WithContext(ctx)

	// 1. 加载旧数据
	acObj, err := sql.Preload(aChannelStore.AlertTemplate).Where(aChannelStore.ID.Eq(int(req.ID))).First()
	if err != nil {
		return err
	}

	// 2. 检查模板是否变更
	// 如果传入的 TemplateID 与数据库中的不一致
	templateChanged := acObj.AlertTemplateID != req.TemplateID

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

	// 更新外键 ID
	acObj.AlertTemplateID = req.TemplateID

	// 【关键修复】如果模板 ID 变了，必须把旧的实体对象清空，否则后续逻辑会认为它还是旧的
	if templateChanged {
		acObj.AlertTemplate = nil
	}

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
		if err := receiver.cache.DelKey(ctx, store.AlertType, acObj.Name); err != nil {
			return err
		}

		if *acObj.Status == model.StatusEnabled {
			// 5. 【修复加载逻辑】
			// 此时由于上面 templateChanged 时设置了 acObj.AlertTemplate = nil
			// 这里的逻辑会正确执行，从数据库拉取最新的模板内容
			if acObj.AlertTemplate == nil {
				template, err := tx.AlertTemplate.WithContext(ctx).Where(aTemlpateStore.ID.Eq(acObj.AlertTemplateID)).First()
				if err != nil {
					return err
				}
				acObj.AlertTemplate = template
			}

			// 6. 存入缓存的是带有最新 AlertTemplate 实体的 acObj
			if err := receiver.cache.SetObject(ctx, store.AlertType, acObj.Name, acObj, store.NeverExpires); err != nil {
				return err
			}
			return receiver.cache.Publish(ctx, constant.AlertChannelTopicUpdate, publish)
		}
		return nil
	})
}

func (receiver *alertChannelService) DeleteChannel(ctx context.Context, req *types.IDRequest) error {
	sql := aChannelStore.WithContext(ctx)

	acObj, err := sql.Where(aChannelStore.ID.Eq(int(req.ID))).First()
	if err != nil {
		return err
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
		_, err := tx.AlertChannel.WithContext(ctx).Unscoped().Where(aChannelStore.ID.Eq(acObj.ID)).Delete(acObj)
		if err != nil {
			return err
		}
		if err := receiver.cache.DelKey(ctx, store.AlertType, acObj.Name); err != nil {
			return err
		}
		return receiver.cache.Publish(ctx, constant.AlertChannelTopicDelete, publish)
	})
}

func (receiver *alertChannelService) QueryChannel(ctx context.Context, req *types.IDRequest) (*model.AlertChannel, error) {
	obj, err := aChannelStore.WithContext(ctx).Where(aChannelStore.ID.Eq(int(req.ID))).First()
	if err != nil {
		return nil, err
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
		sql = sql.Where(aChannelStore.Name.Like("%" + req.Name + "%"))
	} else if req.Type != "" {
		sql.Where(aChannelStore.Type.Like("%" + req.Type + "%"))
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
