package types

type IDRequest struct {
	ID int64 `uri:"id" binding:"required"`
}

type Pagination struct {
	// Page 从 1 开始, 表示第几页
	Page int `form:"page,default=1" json:"page" binding:"omitempty,min=1"`
	// PageSize 最大值 100，默认值 20, 表示每页多少条数据
	PageSize int `form:"pageSize,default=20" json:"pageSize" binding:"omitempty,min=1,max=100"`
}

type ListResponse struct {
	*Pagination
	Total int64 `json:"total"`
}
