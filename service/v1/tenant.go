package v1

import (
	"context"
	"fmt"

	"github.com/alert666/api-server/base/constant"
	"github.com/alert666/api-server/base/helper"
	"github.com/alert666/api-server/base/types"
	"github.com/alert666/api-server/model"
	"github.com/alert666/api-server/store"
	"golang.org/x/sync/singleflight"
)

type TenantServicer interface {
	CreateTenant(ctx context.Context, req *types.TenantCreateRequest) error
	UpdateTenant(ctx context.Context, req *types.TenantUpdateRequest) error
	DeleteTenant(ctx context.Context, req *types.IDRequest) error
	QueryTenant(ctx context.Context, req *types.IDRequest) (*model.Tenant, error)
	ListTenant(ctx context.Context, pagination *types.TenantListRequest) (*types.TenantListResponse, error)
	GetTenantOption(ctx context.Context) ([]types.TenantOption, error)
}

type TenantService struct {
	cacheImpl store.CacheStorer
	sf        singleflight.Group
}

func NewTenantServicer(cacheImpl store.CacheStorer) TenantServicer {
	return &TenantService{
		cacheImpl: cacheImpl,
	}
}

func (receiver *TenantService) CreateTenant(ctx context.Context, req *types.TenantCreateRequest) error {
	count, err := tenantStore.WithContext(ctx).Where(tenantStore.Name.Eq(req.Name)).Count()
	if err != nil {
		return err
	}

	if count > 0 {
		return fmt.Errorf("Tenant %s 已经存在", req.Name)
	}

	return store.Q.Transaction(func(tx *store.Query) error {
		if err := tx.Tenant.WithContext(ctx).Create(&model.Tenant{
			Name:        req.Name,
			Label:       req.Label,
			Description: req.Description,
		}); err != nil {
			return err
		}

		storeObjs, err := tx.Tenant.WithContext(ctx).Find()
		if err != nil {
			return err
		}
		options := make([]*types.TenantOption, 0, len(storeObjs))
		for _, storeObj := range storeObjs {
			options = append(options, &types.TenantOption{
				Label: storeObj.Label,
				Value: storeObj.Name,
			})
		}
		return receiver.cacheImpl.SetObject(ctx, store.TenantType, constant.OptionsCacheKey, options, store.NeverExpires)
	})
}

func (receiver *TenantService) UpdateTenant(ctx context.Context, req *types.TenantUpdateRequest) error {
	return store.Q.Transaction(func(tx *store.Query) error {
		info, err := tenantStore.
			WithContext(ctx).
			Where(tenantStore.ID.Eq(req.ID)).
			UpdateSimple(
				tenantStore.Description.Value(req.Description),
				tenantStore.Label.Value(req.Label),
			)
		if err != nil {
			return err
		}

		if info.RowsAffected == 0 {
			return fmt.Errorf("记录不存在, 更新失败")
		}

		storeObjs, err := tenantStore.WithContext(ctx).Find()
		if err != nil {
			return err
		}

		res := make([]*types.TenantOption, 0, len(storeObjs))
		for _, storeObj := range storeObjs {
			res = append(res, &types.TenantOption{
				Label: storeObj.Label,
				Value: storeObj.Name,
			})
		}

		return receiver.cacheImpl.SetObject(ctx, store.TenantType, constant.OptionsCacheKey, res, store.NeverExpires)
	})

}

func (receiver *TenantService) DeleteTenant(ctx context.Context, req *types.IDRequest) error {
	return store.Q.Transaction(func(tx *store.Query) error {
		info, err := tx.Tenant.WithContext(ctx).Where(tenantStore.ID.Eq(req.ID)).Delete()
		if err != nil {
			return err
		}

		if info.RowsAffected == 0 {
			return fmt.Errorf("记录不存在, 删除失败")
		}

		storeObjs, err := tx.Tenant.WithContext(ctx).Find()
		if err != nil {
			return err
		}
		options := make([]*types.TenantOption, 0, len(storeObjs))
		for _, storeObj := range storeObjs {
			options = append(options, &types.TenantOption{
				Label: storeObj.Name,
				Value: storeObj.Name,
			})
		}
		return receiver.cacheImpl.SetObject(ctx, store.TenantType, constant.OptionsCacheKey, options, store.NeverExpires)
	})
}

func (receiver *TenantService) QueryTenant(ctx context.Context, req *types.IDRequest) (*model.Tenant, error) {
	return tenantStore.WithContext(ctx).Where(tenantStore.ID.Eq(req.ID)).First()
}

func (receiver *TenantService) ListTenant(ctx context.Context, req *types.TenantListRequest) (*types.TenantListResponse, error) {
	var (
		Tenants []*model.Tenant
		total   int64
		query   = tenantStore.WithContext(ctx)
		err     error
	)
	if req.Name != "" {
		query = query.Where(tenantStore.Name.Like("%" + req.Name + "%"))
	}

	if total, err = query.Count(); err != nil {
		return nil, err
	}

	if req.Sort != "" && req.Direction != "" {
		sort, ok := tenantStore.GetFieldByName(req.Sort)
		if !ok {
			return nil, fmt.Errorf("invalid sort field: %s", req.Sort)
		}
		query = query.Order(helper.Sort(sort, req.Direction))
	} else {
		query = query.Order(tenantStore.CreatedAt.Desc())
	}

	if req.PageSize == 0 || req.Page == 0 {
		return nil, fmt.Errorf("page and pageSize must be greater than 0")
	}

	if Tenants, err = query.Limit(req.PageSize).Offset((req.Page - 1) * req.PageSize).Find(); err != nil {
		return nil, err
	}

	return types.NewTenantListResponse(Tenants, total, req.PageSize, req.Page), nil
}

func (receiver *TenantService) GetTenantOption(ctx context.Context) ([]types.TenantOption, error) {
	var res []types.TenantOption

	exists, err := receiver.cacheImpl.GetObject(ctx, store.TenantType, constant.OptionsCacheKey, &res)
	if err != nil {
		return nil, err
	}
	if exists {
		return res, nil
	}

	data, err, _ := receiver.sf.Do(constant.OptionsCacheKey, func() (interface{}, error) {
		var innerRes []*types.TenantOption
		if ex, _ := receiver.cacheImpl.GetObject(ctx, store.TenantType, constant.OptionsCacheKey, &innerRes); ex {
			return innerRes, nil
		}

		var storeObjs []struct {
			Name  string
			Label string
		}
		err := tenantStore.WithContext(ctx).
			Select(tenantStore.Name, tenantStore.Label).
			Scan(&storeObjs)
		if err != nil {
			return nil, err
		}

		resList := make([]types.TenantOption, 0, len(storeObjs))
		for _, obj := range storeObjs {
			label := obj.Label
			if label == "" {
				label = obj.Name
			}
			resList = append(resList, types.TenantOption{
				Label: label,
				Value: obj.Name,
			})
		}

		// 5. 设置缓存（即使 resList 为空也设置，避免缓存穿透）
		_ = receiver.cacheImpl.SetObject(ctx, store.TenantType, constant.OptionsCacheKey, resList, store.NeverExpires)

		return resList, nil
	})

	if err != nil {
		return nil, err
	}

	return data.([]types.TenantOption), nil
}
