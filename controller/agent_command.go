package controller

import (
	"github.com/alert666/api-server/base/bind"
	v1 "github.com/alert666/api-server/service/v1"
	"github.com/gin-gonic/gin"
)

type AgentCommandController interface {
	SendCommandAndWait(c *gin.Context)
}

type agentCommandController struct {
	dataTunnelSvc v1.DataTunnelServicer
}

func NewAgentCommandController(dataTunnelSvc v1.DataTunnelServicer) AgentCommandController {
	return &agentCommandController{
		dataTunnelSvc: dataTunnelSvc,
	}
}

func (ctrl *agentCommandController) SendCommandAndWait(c *gin.Context) {
	bind.ResponseWithData(c, ctrl.dataTunnelSvc.SendCommandAndWait, bind.BindTypeShouldBind)
}
