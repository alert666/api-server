package controller

import (
	"github.com/alert666/api-server/base/bind"
	v1 "github.com/alert666/api-server/service/v1"
	"github.com/gin-gonic/gin"
)

// InternalForwardController handles cross-replica command forwarding.
type InternalForwardController struct {
	dataTunnelSvc v1.DataTunnelServicer
}

func NewInternalForwardController(dataTunnelSvc v1.DataTunnelServicer) *InternalForwardController {
	return &InternalForwardController{dataTunnelSvc: dataTunnelSvc}
}

// HandleForward receives a forwarded command and executes it locally.
func (c *InternalForwardController) HandleForward(ctx *gin.Context) {
	bind.ResponseWithData(ctx, c.dataTunnelSvc.ExecuteCommandLocally, bind.BindTypeShouldBind)
}