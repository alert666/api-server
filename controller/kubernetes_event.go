package controller

import (
	"github.com/alert666/api-server/base/bind"
	v1 "github.com/alert666/api-server/service/v1"
	"github.com/gin-gonic/gin"
)

type KubernetesEventController interface {
	ReceiveEvent(*gin.Context)
	QueryImagePullDuration(*gin.Context)
}

type kubernetesEventController struct {
	service v1.KubernetesEventServicer
}

func NewKubernetesEventController(service v1.KubernetesEventServicer) KubernetesEventController {
	return &kubernetesEventController{service: service}
}

// ReceiveEvent receives and persists a kube-eventer webhook payload.
// @Summary 接收 Kubernetes Event
// @Tags Kubernetes Event
// @Accept json
// @Produce json
// @Param X-Tenant-Id header string true "集群名称"
// @Param data body types.KubernetesEventReceiveReq true "Kubernetes Event"
// @Success 200 {object} types.Response
// @Security BearerAuth
// @Router /api/v1/kubernetesEvent [post]
func (c *kubernetesEventController) ReceiveEvent(ctx *gin.Context) {
	bind.ResponseOnlySuccess(ctx, c.service.ReceiveEvent, bind.BindTypeJson)
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
// @Router /api/v1/kubernetesEvent/getPulledImageDuration [get]
func (c *kubernetesEventController) QueryImagePullDuration(ctx *gin.Context) {
	bind.ResponseWithData(ctx, c.service.QueryImagePullDuration, bind.BindTypeQuery)
}
