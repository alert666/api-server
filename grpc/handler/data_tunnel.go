package handler

import (
	"context"
	"io"

	pb "github.com/alert666/alertmanager-proto/gen/go/data_tunnel/v1"
	svc "github.com/alert666/api-server/service/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TunnelHandler 实现 proto 定义的 TunnelServiceServer
type TunnelHandler struct {
	pb.UnimplementedTunnelServiceServer
	dataTunnelSvc svc.DataTunnelServicer
}

// NewTunnelHandler 创建 TunnelHandler
func NewTunnelHandler(dataTunnelSvc svc.DataTunnelServicer) *TunnelHandler {
	return &TunnelHandler{
		dataTunnelSvc: dataTunnelSvc,
	}
}

// DataTunnel 双向流 RPC 实现
// 客户端通过此 RPC 与服务端建立长连接，收发 TunnelMessage
func (h *TunnelHandler) DataTunnel(stream pb.TunnelService_DataTunnelServer) error {
	ctx := stream.Context()

	// 1. 接收首条消息，必须是 Init
	msg, err := stream.Recv()
	if err != nil {
		zap.L().Error("data tunnel recv init failed", zap.Error(err))
		return status.Errorf(codes.Internal, "recv init failed: %v", err)
	}

	init := msg.GetInit()
	if init == nil {
		return status.Error(codes.InvalidArgument, "first message must be Init")
	}

	agentID := init.GetAgentId()
	if agentID == "" {
		return status.Error(codes.InvalidArgument, "agent_id is required")
	}

	zap.L().Info("agent connected",
		zap.String("agent_id", agentID),
		zap.String("cluster_id", init.GetClusterId()),
		zap.String("version", init.GetVersion()),
	)

	// 2. 注册 agent
	if err := h.dataTunnelSvc.RegisterAgent(ctx, init); err != nil {
		zap.L().Error("register agent failed", zap.String("agent_id", agentID), zap.Error(err))
		return status.Errorf(codes.Internal, "register agent failed: %v", err)
	}
	defer func() {
		if err := h.dataTunnelSvc.UnregisterAgent(context.Background(), agentID); err != nil {
			zap.L().Error("unregister agent failed", zap.String("agent_id", agentID), zap.Error(err))
		}
	}()

	// 3. 启动 goroutine 持续接收消息
	errCh := make(chan error, 1)
	go func() {
		for {
			msg, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					errCh <- nil
				} else {
					errCh <- err
				}
				return
			}

			if result := msg.GetCommandResult(); result != nil {
				if err := h.dataTunnelSvc.HandleCommandResult(ctx, agentID, result); err != nil {
					zap.L().Error("handle command result failed",
						zap.String("agent_id", agentID),
						zap.Int32("cmd", int32(result.GetCommandType())),
						zap.Error(err),
					)
				}
			}
		}
	}()

	// 4. 主循环：下发命令
	for {
		select {
		case <-ctx.Done():
			zap.L().Info("agent disconnected", zap.String("agent_id", agentID))
			return ctx.Err()
		case err := <-errCh:
			if err != nil && err != io.EOF {
				zap.L().Error("stream recv error", zap.String("agent_id", agentID), zap.Error(err))
			}
			return err
		default:
		}

		commands, err := h.dataTunnelSvc.PullCommands(ctx, agentID)
		if err != nil {
			zap.L().Error("pull commands failed", zap.String("agent_id", agentID), zap.Error(err))
			continue
		}

		for _, cmd := range commands {
			msg := &pb.TunnelMessage{
				RequestId: agentID,
				Payload:   &pb.TunnelMessage_Command{Command: cmd},
			}
			if err := stream.Send(msg); err != nil {
				zap.L().Error("send command failed",
					zap.String("agent_id", agentID),
					zap.Int32("cmd", int32(cmd.GetType())),
					zap.Error(err),
				)
				return err
			}
		}

		// 频率控制
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
}
