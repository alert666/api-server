package controller

import (
	v1 "github.com/alert666/api-server/service/v1"
	"github.com/gin-gonic/gin"
)

type ClusterController interface {
	CreateCluster(c *gin.Context)
	UpdateCluster(c *gin.Context)
	DeleteCluster(c *gin.Context)
	QueryCluster(c *gin.Context)
	ListCluster(c *gin.Context)
	GetClusterOption(c *gin.Context)
}

type clusterController struct {
	ClusterHistoryService v1.TenantServicer
}

func NewClusterController(ClusterHistoryService v1.TenantServicer) ClusterController {
	return &clusterController{
		ClusterHistoryService: ClusterHistoryService,
	}
}

// CreateCluster 创建集群
// @Summary 创建集群
// @Description 创建集群
// @Tags 集群管理
// @Accept json
// @Produce json
// @Param data body types.ClusterCreateRequest true "创建请求参数"
// @Success 200 {object} types.Response "创建成功"
// @Router /api/v1/cluster [post]
func (recevicer *clusterController) CreateCluster(c *gin.Context) {
	ResponseOnlySuccess(c, recevicer.ClusterHistoryService.CreateTenant, bindTypeJson)
}

// UpdateCluster 更新集群
// @Summary 更新集群
// @Description 更新集群
// @Tags 集群管理
// @Accept json
// @Produce json
// @Param data body types.ClusterUpdateRequest true "更新请求参数"
// @Success 200 {object} types.Response "更新成功"
// @Router /api/v1/Cluster/:id [put]
func (recevicer *clusterController) UpdateCluster(c *gin.Context) {
	ResponseOnlySuccess(c, recevicer.ClusterHistoryService.UpdateTenant, bindTypeJson, bindTypeUri)
}

// DeleteCluster 删除集群
// @Summary 删除集群
// @Description 删除集群
// @Tags 集群管理
// @Accept json
// @Produce json
// @Param data body types.IDRequest true "删除请求参数"
// @Success 200 {object} types.Response "删除成功"
// @Router /api/v1/Cluster/:id [delete]
func (recevicer *clusterController) DeleteCluster(c *gin.Context) {
	ResponseOnlySuccess(c, recevicer.ClusterHistoryService.DeleteTenant, bindTypeUri)
}

// QueryCluster 查询集群
// @Summary 查询集群
// @Description 查询集群
// @Tags 集群管理
// @Accept json
// @Produce json
// @Param data body types.IDRequest true "查询请求参数"
// @Success 200 {object} types.Response{data=model.Cluster} "查询成功"
// @Router /api/v1/cluster/:id [get]
func (recevicer *clusterController) QueryCluster(c *gin.Context) {
	ResponseWithData(c, recevicer.ClusterHistoryService.QueryTenant, bindTypeUri)
}

// ListCluster 集群列表
// @Summary 集群列表
// @Description 使用分页查询集群的信息, 支持根据 name 查询
// @Tags 集群管理
// @Accept json
// @Produce json
// @Param data query types.ClusterListRequest true "查询请求参数"
// @Success 200 {object} types.Response{data=types.ClusterListResponse} "查询成功"
// @Router /api/v1/cluster/ [get]
func (recevicer *clusterController) ListCluster(c *gin.Context) {
	ResponseWithData(c, recevicer.ClusterHistoryService.ListTenant, bindTypeQuery)
}

// GetClusterOption 获取集群 Options
// @Summary 获取集群 Options
// @Description 获取集群 Options
// @Tags 集群管理
// @Accept json
// @Produce json
// @Success 200 {object} types.Response{data=[]types.ClusterOption} "查询成功"
// @Router /api/v1/cluster/options [get]
func (recevicer *clusterController) GetClusterOption(c *gin.Context) {
	ResponseWithDataNoBind(c, recevicer.ClusterHistoryService.GetTenantOption)
}
