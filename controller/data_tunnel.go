package controller

import (
	"github.com/alert666/api-server/base/bind"
	v1 "github.com/alert666/api-server/service/v1"
	"github.com/gin-gonic/gin"
)

type DataTunnelController interface {
	SendCommandAndWait(c *gin.Context)
	PrometheusProbe(c *gin.Context)
}

type dataTunnelController struct {
	dataTunnelImpl v1.DataTunnelServicer
	prometheusImpl v1.Prometheuser
}

func NewDataTunnelController(dataTunnelImpl v1.DataTunnelServicer, prometheusImpl v1.Prometheuser) DataTunnelController {
	return &dataTunnelController{
		dataTunnelImpl: dataTunnelImpl,
		prometheusImpl: prometheusImpl,
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
func (ctrl *dataTunnelController) SendCommandAndWait(c *gin.Context) {
	bind.ResponseWithData(c, ctrl.dataTunnelImpl.SendCommandAndWait, bind.BindTypeShouldBind)
}

// PrometheusProbe 探测 idc 机房 Prometheus 健康状态
// @Summary 向 Agent 下发命令，探测机房 Prometheus 健康状态
// @Description 通过 gRPC 数据隧道向指定 Agent 发送命令，阻塞等待执行结果后返回。
// @Tags Agent 命令
// @Accept json
// @Produce json
// @Success 200 {object} types.Response "命令执行成功"
// @Router /api/v1/agents/commands/prometheusProbe [get]
func (ctrl *dataTunnelController) PrometheusProbe(c *gin.Context) {
	bind.ResponseWithDataNoBind(c, ctrl.prometheusImpl.PrometheusProbe)
}
