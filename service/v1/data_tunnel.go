package v1

import (
	"context"

	pb "github.com/alert666/alertmanager-proto/gen/go/data_tunnel/v1"
)

// DataTunnelServicer 数据隧道服务接口
// 负责管理 agent 注册、命令下发和结果处理
type DataTunnelServicer interface {
	// RegisterAgent 注册 agent 连接
	RegisterAgent(ctx context.Context, init *pb.Init) error
	// UnregisterAgent 注销 agent 连接
	UnregisterAgent(ctx context.Context, agentID string) error
	// HandleCommandResult 处理命令执行结果
	HandleCommandResult(ctx context.Context, agentID string, result *pb.CommandResult) error
	// PullCommands 拉取 agent 待执行的命令
	PullCommands(ctx context.Context, agentID string) ([]*pb.Command, error)
}

// DataTunnelService 数据隧道服务默认实现
type DataTunnelService struct{}

// NewDataTunnelService 创建 DataTunnelService
func NewDataTunnelService() DataTunnelServicer {
	return &DataTunnelService{}
}

func (s *DataTunnelService) RegisterAgent(_ context.Context, _ *pb.Init) error {
	return nil
}

func (s *DataTunnelService) UnregisterAgent(_ context.Context, _ string) error {
	return nil
}

func (s *DataTunnelService) HandleCommandResult(_ context.Context, _ string, _ *pb.CommandResult) error {
	return nil
}

func (s *DataTunnelService) PullCommands(_ context.Context, _ string) ([]*pb.Command, error) {
	return nil, nil
}
