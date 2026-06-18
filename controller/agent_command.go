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

// SendCommandAndWait 向 Agent 下发命令并等待结果
// @Summary 向 Agent 下发命令
// @Description 通过 gRPC 数据隧道向指定 Agent 发送命令，阻塞等待执行结果后返回。
// @Tags Agent 命令
// @Accept json
// @Produce json
// @Param data body types.SendCommandAndWaitReq true "命令请求参数"
// @Success 200 {object} types.Response "命令执行成功"
// @Router /api/v1/agents/commands/wait [post]
func (ctrl *agentCommandController) SendCommandAndWait(c *gin.Context) {
	bind.ResponseWithData(c, ctrl.dataTunnelSvc.SendCommandAndWait, bind.BindTypeShouldBind)
}
