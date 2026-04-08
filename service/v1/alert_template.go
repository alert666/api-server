package v1

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/alert666/api-server/base/helper"
	"github.com/alert666/api-server/base/types"
	"github.com/alert666/api-server/model"
	"github.com/alert666/api-server/store"
	"gorm.io/gorm"
)

type AlertTemplateServicer interface {
	CreateAlerTemplate(ctx context.Context, req *types.AlertTemplateCreateRequest) error
	UpdateTemplate(ctx context.Context, req *types.AlertTemplateUpdateRequest) error
	DeleteTemplate(ctx context.Context, req *types.IDRequest) error
	QueryTemplate(ctx context.Context, req *types.IDRequest) (*model.AlertTemplate, error)
	ListTemplate(ctx context.Context, req *types.AlertTemplateListRequest) (*types.AlertTemplateListResponse, error)
}

type alertTemplateService struct {
	cache store.CacheStorer
}

func NewAlertTemplateServicer(cache store.CacheStorer) AlertTemplateServicer {
	return &alertTemplateService{
		cache: cache,
	}
}

func (receiver *alertTemplateService) CreateAlerTemplate(ctx context.Context, req *types.AlertTemplateCreateRequest) error {
	storeObj, err := aTemlpate.WithContext(ctx).Where(aTemlpate.Name.Eq(req.Name)).First()
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if storeObj != nil {
		return fmt.Errorf("%s AlertTemplate 已经存在, 创建失败", req.Name)
	}

	templateBy, err := base64.StdEncoding.DecodeString(req.Template)
	if err != nil {
		return fmt.Errorf("base64 解密 Template 失败, %s", err)
	}

	var aggTemplateBy []byte
	if req.AggregationTemplate != "" {
		aggTemplateBy, err = base64.StdEncoding.DecodeString(req.AggregationTemplate)
		if err != nil {
			return fmt.Errorf("base64 解密 AggregationTemplate 失败, %s", err)
		}
	}

	saveObj := &model.AlertTemplate{
		Name:                req.Name,
		Description:         req.Description,
		Template:            string(templateBy),
		AggregationTemplate: string(aggTemplateBy),
	}

	if req.AggregationTemplate != "" {
		if err := helper.ValidateYamlTemplate(ctx, true, saveObj.AggregationTemplate); err != nil {
			return fmt.Errorf("测试聚合模板失败, %s", err)
		}
	}

	if req.Template != "" {
		if err := helper.ValidateYamlTemplate(ctx, false, saveObj.Template); err != nil {
			return fmt.Errorf("测试非聚合模板失败, %s", err)
		}
	}

	return aTemlpate.WithContext(ctx).Create(saveObj)
}

func (receiver *alertTemplateService) UpdateTemplate(ctx context.Context, req *types.AlertTemplateUpdateRequest) error {
	obj, err := aTemlpate.WithContext(ctx).Where(aTemlpate.ID.Eq(int(req.ID))).First()
	if err != nil {
		return err
	}

	templateBy, err := base64.StdEncoding.DecodeString(req.Template)
	if err != nil {
		return fmt.Errorf("base64 解密 Template 失败, %s", err)
	}

	var aggTemplateBy []byte
	if req.AggregationTemplate != "" {
		aggTemplateBy, err = base64.StdEncoding.DecodeString(req.AggregationTemplate)
		if err != nil {
			return fmt.Errorf("base64 解密 AggregationTemplate 失败, %s", err)
		}
	}

	obj.Template = string(templateBy)
	obj.AggregationTemplate = string(aggTemplateBy)
	obj.Description = req.Description

	if req.AggregationTemplate != "" {
		if err := helper.ValidateYamlTemplate(ctx, true, obj.AggregationTemplate); err != nil {
			return fmt.Errorf("测试聚合模板失败, %s", err)
		}
	}

	if req.Template != "" {
		if err := helper.ValidateYamlTemplate(ctx, false, obj.Template); err != nil {
			return fmt.Errorf("测试聚合模板失败, %s", err)
		}
	}

	var acObj *model.AlertChannel
	acObj, err = aChannel.WithContext(ctx).Where(aChannel.AlertTemplateID.Eq(int(req.ID))).First()
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		// 如果 channel 不存在, 直接更新 template 即可
		return aTemlpate.WithContext(ctx).Save(obj)
	}

	return store.Q.Transaction(func(tx *store.Query) error {
		if acObj == nil {
			return fmt.Errorf("更新模版失败, 关联的 AlertChannel 不存在")
		}
		acObj.AlertTemplate = obj
		if err := receiver.cache.DelKey(ctx, store.AlertType, acObj.Name); err != nil {
			return err
		}
		if err := receiver.cache.SetObject(ctx, store.AlertType, acObj.Name, acObj, store.NeverExpires); err != nil {
			return err
		}
		return tx.AlertTemplate.WithContext(ctx).Save(obj)
	})
}

func (receiver *alertTemplateService) DeleteTemplate(ctx context.Context, req *types.IDRequest) error {
	_, err := aTemlpate.WithContext(ctx).Where(aTemlpate.ID.Eq(int(req.ID))).First()
	if err != nil {
		return err
	}

	acObj, err := aChannel.WithContext(ctx).Where(aChannel.AlertTemplateID.Eq(int(req.ID))).First()
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if err == nil {
		return fmt.Errorf("当前模板已绑定 [%s] 告警通道，请先解除绑定", acObj.Name)
	}

	if _, err := aTemlpate.WithContext(ctx).Where(aTemlpate.ID.Eq(int(req.ID))).Delete(); err != nil {
		return err
	}
	return nil
}

func (receiver *alertTemplateService) QueryTemplate(ctx context.Context, req *types.IDRequest) (*model.AlertTemplate, error) {
	obj, err := aTemlpate.WithContext(ctx).Where(aTemlpate.ID.Eq(int(req.ID))).First()
	if err != nil {
		return nil, err
	}

	return obj, nil
}

func (receiver *alertTemplateService) ListTemplate(ctx context.Context, req *types.AlertTemplateListRequest) (*types.AlertTemplateListResponse, error) {
	var (
		alertTemplates []*model.AlertTemplate
		total          int64
		sql            = aTemlpate.WithContext(ctx)
		err            error
	)

	if req.Name != "" {
		sql = sql.Where(aTemlpate.Name.Like("%" + req.Name + "%"))
	}

	if total, err = sql.Count(); err != nil {
		return nil, err
	}

	if req.Sort != "" && req.Direction != "" {
		sort, ok := aTemlpate.GetFieldByName(req.Sort)
		if !ok {
			return nil, fmt.Errorf("invalid sort field: %s", req.Sort)
		}
		sql = sql.Order(helper.Sort(sort, req.Direction))
	}

	if req.PageSize == 0 || req.Page == 0 {
		if alertTemplates, err = sql.Find(); err != nil {
			return nil, err
		}
	} else {
		if alertTemplates, err = sql.Limit(req.PageSize).Offset((req.Page - 1) * req.PageSize).Find(); err != nil {
			return nil, err
		}
	}
	return types.NewAlertTemplateListResponse(alertTemplates, total, req.PageSize, req.Page), nil
}
