package v1

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"go.uber.org/zap"

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
	CopyTemplate(ctx context.Context, req *types.AlertTemplateCopyRequest) (*model.AlertTemplate, error)
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
	storeObj, err := aTemlpateStore.WithContext(ctx).Where(aTemlpateStore.Name.Eq(req.Name)).First()
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if storeObj != nil {
		return fmt.Errorf("%s AlertTemplate 已经存在, 创建失败", req.Name)
	}

	if req.AlertChannelID > 0 {
		// 校验关联的告警渠道是否存在且已启用
		channel, err := aChannelStore.WithContext(ctx).Where(aChannelStore.ID.Eq(req.AlertChannelID)).First()
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("关联的告警渠道(ID=%d)不存在", req.AlertChannelID)
			}
			return fmt.Errorf("查询告警渠道失败: %w", err)
		}
		if *channel.Status != model.StatusEnabled {
			return fmt.Errorf("关联的告警渠道 [%s] 未启用", channel.Name)
		}
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
		AlertChannelID:      req.AlertChannelID,
		ReceiveIdType:       req.ReceiveIdType,
		ReceiveId:           req.ReceiveId,
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

	if err := helper.ValidateTemplateRecipient(req.ReceiveIdType, req.ReceiveId); err != nil {
		return fmt.Errorf("接收者配置校验失败: %s", err)
	}

	if err := aTemlpateStore.WithContext(ctx).Create(saveObj); err != nil {
		return err
	}

	// 缓存新创建的模板
	if err := receiver.cache.SetObject(ctx, store.AlertTemplateType, saveObj.Name, saveObj, store.NeverExpires); err != nil {
		zap.L().Error("cache AlertTemplate failed", zap.String("name", saveObj.Name), zap.Error(err))
	}
	return nil
}

func (receiver *alertTemplateService) UpdateTemplate(ctx context.Context, req *types.AlertTemplateUpdateRequest) error {
	obj, err := aTemlpateStore.WithContext(ctx).Where(aTemlpateStore.ID.Eq(int(req.ID))).First()
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
	obj.ReceiveIdType = req.ReceiveIdType
	obj.ReceiveId = req.ReceiveId
	obj.AlertChannelID = req.AlertChannelID
	obj.Description = req.Description

	if err := helper.ValidateTemplateRecipient(req.ReceiveIdType, req.ReceiveId); err != nil {
		return fmt.Errorf("接收者配置校验失败: %s", err)
	}

	// 校验关联的告警渠道是否存在且已启用
	if req.AlertChannelID > 0 {
		channel, err := aChannelStore.WithContext(ctx).Where(aChannelStore.ID.Eq(req.AlertChannelID)).First()
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("关联的告警渠道(ID=%d)不存在", req.AlertChannelID)
			}
			return fmt.Errorf("查询告警渠道失败: %w", err)
		}
		if *channel.Status != model.StatusEnabled {
			return fmt.Errorf("关联的告警渠道 [%s] 未启用", channel.Name)
		}
	}

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

	if err := aTemlpateStore.WithContext(ctx).Save(obj); err != nil {
		return err
	}
	// 更新缓存
	if err := receiver.cache.SetObject(ctx, store.AlertTemplateType, obj.Name, obj, store.NeverExpires); err != nil {
		zap.L().Error("cache AlertTemplate update failed", zap.String("name", obj.Name), zap.Error(err))
	}
	return nil
}

func (receiver *alertTemplateService) DeleteTemplate(ctx context.Context, req *types.IDRequest) error {
	obj, err := aTemlpateStore.WithContext(ctx).Where(aTemlpateStore.ID.Eq(int(req.ID))).First()
	if err != nil {
		return err
	}

	if _, err := aTemlpateStore.WithContext(ctx).Where(aTemlpateStore.ID.Eq(int(req.ID))).Delete(); err != nil {
		return err
	}

	if err := receiver.cache.DelKey(ctx, store.AlertTemplateType, obj.Name); err != nil {
		zap.L().Error("delete AlertTemplate cache failed", zap.String("name", obj.Name), zap.Error(err))
	}
	return nil

}

func (receiver *alertTemplateService) CopyTemplate(ctx context.Context, req *types.AlertTemplateCopyRequest) (*model.AlertTemplate, error) {
	// 查询源模板
	src, err := aTemlpateStore.WithContext(ctx).Where(aTemlpateStore.ID.Eq(int(req.ID))).First()
	if err != nil {
		return nil, err
	}

	// 检查用户指定的名称是否已存在
	newName := req.Name
	exist, err := aTemlpateStore.WithContext(ctx).Where(aTemlpateStore.Name.Eq(newName)).First()
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if exist != nil {
		return nil, fmt.Errorf("%s AlertTemplate 已经存在, 拷贝失败", newName)
	}

	// 拷贝字段构建新对象（不设置 ID/CreatedAt/UpdatedAt，由 DB 自动生成）
	newObj := &model.AlertTemplate{
		Name:                newName,
		AlertChannelID:      src.AlertChannelID,
		ReceiveIdType:       src.ReceiveIdType,
		ReceiveId:           src.ReceiveId,
		Description:         src.Description,
		Template:            src.Template,
		AggregationTemplate: src.AggregationTemplate,
	}

	// 校验接收者配置
	if err := helper.ValidateTemplateRecipient(src.ReceiveIdType, src.ReceiveId); err != nil {
		return nil, fmt.Errorf("接收者配置校验失败: %s", err)
	}

	if err := aTemlpateStore.WithContext(ctx).Create(newObj); err != nil {
		return nil, err
	}

	// 缓存
	if err := receiver.cache.SetObject(ctx, store.AlertTemplateType, newObj.Name, newObj, store.NeverExpires); err != nil {
		zap.L().Error("cache copied AlertTemplate failed", zap.String("name", newObj.Name), zap.Error(err))
	}
	return newObj, nil
}

func (receiver *alertTemplateService) QueryTemplate(ctx context.Context, req *types.IDRequest) (*model.AlertTemplate, error) {
	obj, err := aTemlpateStore.WithContext(ctx).Where(aTemlpateStore.ID.Eq(int(req.ID))).First()
	if err != nil {
		return nil, err
	}

	return obj, nil
}

func (receiver *alertTemplateService) ListTemplate(ctx context.Context, req *types.AlertTemplateListRequest) (*types.AlertTemplateListResponse, error) {
	var (
		alertTemplates []*model.AlertTemplate
		total          int64
		sql            = aTemlpateStore.WithContext(ctx)
		err            error
	)

	if req.Name != "" {
		sql = sql.Where(aTemlpateStore.Name.Like("%" + req.Name + "%"))
	}

	if total, err = sql.Count(); err != nil {
		return nil, err
	}

	if req.Sort != "" && req.Direction != "" {
		sort, ok := aTemlpateStore.GetFieldByName(req.Sort)
		if !ok {
			return nil, fmt.Errorf("invalid sort field: %s", req.Sort)
		}
		sql = sql.Order(helper.Sort(sort, req.Direction))
	} else {
		sql = sql.Order(aTemlpateStore.CreatedAt.Desc())
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
