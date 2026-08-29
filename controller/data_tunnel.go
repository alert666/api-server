package controller

import (
	"github.com/alert666/api-server/base/bind"
	v1 "github.com/alert666/api-server/service/v1"
	"github.com/gin-gonic/gin"
)

type DataTunnelController interface {
	SendCommandAndWait(c *gin.Context)
	ClusterProbe(c *gin.Context)
}

type dataTunnelController struct {
	dataTunnelImpl    v1.DataTunnelServicer
	clusterProberImpl v1.ClusterProber
}

func NewDataTunnelController(dataTunnelImpl v1.DataTunnelServicer, clusterProberImpl v1.ClusterProber) DataTunnelController {
	return &dataTunnelController{
		dataTunnelImpl:    dataTunnelImpl,
		clusterProberImpl: clusterProberImpl,
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
func (dtc *dataTunnelController) SendCommandAndWait(c *gin.Context) {
	bind.ResponseWithData(c, dtc.dataTunnelImpl.SendCommandAndWait, bind.BindTypeShouldBind)
}

// ClusterProbe 探测集群指定端点健康状态
// @Summary 探测集群指定端点健康状态
// @Description 通过 gRPC 数据隧道向指定 Agent 发送集群探测命令，阻塞等待执行结果后返回。
// @Tags Agent 命令
// @Accept json
// @Produce json
// @Param data query types.ClusterProbeReq true "集群探测请求参数"
// @Success 200 {object} types.Response "命令执行成功"
// @Router /api/v1/agents/commands/clusterProbe [get]
func (dtc *dataTunnelController) ClusterProbe(c *gin.Context) {
	bind.ResponseWithData(c, dtc.clusterProberImpl.ClusterProbe, bind.BindTypeQuery)
}
