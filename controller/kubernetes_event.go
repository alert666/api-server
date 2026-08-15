package controller

import (
	"github.com/alert666/api-server/base/bind"
	v1 "github.com/alert666/api-server/service/v1"
	"github.com/gin-gonic/gin"
)

type KubernetesEventController interface {
	ReceiveEvent(*gin.Context)
}

type kubernetesEventController struct {
	KubernetesEventServicer v1.KubernetesEventServicer
}

func NewKubernetesEventController(service v1.KubernetesEventServicer) KubernetesEventController {
	return &kubernetesEventController{KubernetesEventServicer: service}
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
	bind.ResponseOnlySuccess(ctx, c.KubernetesEventServicer.ReceiveEvent, bind.BindTypeJson)
}
