package controller

import (
	"github.com/alert666/api-server/base/bind"
	v1 "github.com/alert666/api-server/service/v1"
	"github.com/gin-gonic/gin"
)

// IDCMetricsController 定制功能
type IDCMetricsController interface {
	GetIDCMetricser(ctx *gin.Context)
	QueryIDCMetricser(ctx *gin.Context)
	QueryImagePullDuration(ctx *gin.Context)
}

type idcMetricsController struct {
	idcMetricser v1.IDCMetricser
}

func NewIDCMetrics(idcMetricser v1.IDCMetricser) IDCMetricsController {
	return &idcMetricsController{
		idcMetricser: idcMetricser,
	}
}

// GetIDCMetricser 查询指定时间 idc 机房的指标数据
// @Summary 查询指定时间 idc 机房的指标数据
// @Description 查询指定时间范围内，idc 机房的指标数据，如节点 notReady gpu 掉卡等
// @Tags IDC Metricser
// @Accept json
// @Produce json
// @Param startTimestamp query integer true "开始时间戳" Example(1721779200)
// @Param endTimestamp query integer true "结束时间戳" Example(1721865599)
// @Param alertName query string true "告警名称" Example(KubeNodeNotReady)
// @Success 200 {object} types.Response{data=types.QeryIDCMetricsRes} "查询成功"
// @Security BearerAuth
// @Router /api/v1/idcMetrics [get]
func (idc *idcMetricsController) GetIDCMetricser(ctx *gin.Context) {
	bind.ResponseWithData(ctx, idc.idcMetricser.GetIDCMetrics, bind.BindTypeQuery)
}

// QueryIDCMetricser 查询指定条件 IDC 机房的指标数据
// @Summary 查询 IDC 机房指标数据
// @Description 根据告警名称、时间范围、节点和IP地址查询IDC机房的指标数据，支持节点NotReady、GPU掉卡等告警类型
// @Tags IDC Metrics
// @Accept json
// @Produce json
// @Param request body types.QueryIDCMetricsReq true "查询请求参数"
// @Success 200 {object} types.Response{data=types.QueryIDCMetricsRes} "查询成功"
// @Failure 400 {object} types.Response "请求参数错误"
// @Failure 401 {object} types.Response "未授权"
// @Failure 404 {object} types.Response "未找到数据"
// @Failure 500 {object} types.Response "服务器内部错误"
// @Security BearerAuth
// @Router /api/v1/idc/idcMetrics [post]
func (idc *idcMetricsController) QueryIDCMetricser(ctx *gin.Context) {
	bind.ResponseWithData(ctx, idc.idcMetricser.QueryIDCMetrics, bind.BindTypeJson)
}

// QueryImagePullDuration 查询指定时间内镜像拉取耗时统计
// @Summary 查询镜像拉取耗时统计
// @Description 查询指定时间范围内，Kubernetes 集群中镜像拉取的耗时统计信息，包括镜像名称、拉取耗时、镜像大小等
// @Tags Kubernetes Event
// @Accept json
// @Produce json
// @Param startTimestamp query integer true "开始时间戳" Example(1721779200)
// @Param endTimestamp query integer true "结束时间戳" Example(1721865599)
// @Success 200 {object} types.Response{data=types.QueryImagePullDurationRes} "查询成功"
// @Security BearerAuth
// @Router /api/v1/idcMetrics/getPulledImageDuration [get]
func (idc *idcMetricsController) QueryImagePullDuration(ctx *gin.Context) {
	bind.ResponseWithData(ctx, idc.idcMetricser.QueryImagePullDuration, bind.BindTypeQuery)
}
