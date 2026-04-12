package types

import "github.com/alert666/api-server/model"

type TenantCreateRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

type TenantUpdateRequest struct {
	*IDRequest
	Description string `json:"description"`
}

type TenantListRequest struct {
	*Pagination
	Name      string `form:"name"`
	Sort      string `form:"sort" binding:"omitempty,oneof=id name created_at updated_at"`
	Direction string `form:"direction" binding:"omitempty,oneof=asc desc"`
}

type TenantListResponse struct {
	*ListResponse
	List []*model.Tenant `json:"list"`
}

func NewTenantListResponse(tenants []*model.Tenant, total int64, pageSize, page int) *TenantListResponse {
	return &TenantListResponse{
		ListResponse: &ListResponse{
			Total: total,
			Pagination: &Pagination{
				Page:     page,
				PageSize: pageSize,
			},
		},
		List: tenants,
	}
}

type TenantOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}
