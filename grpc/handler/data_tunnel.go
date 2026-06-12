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

	agentID := init.GetAgentID()
	clusterID := init.GetClusterID()
	if agentID == "" {
		return status.Error(codes.InvalidArgument, "agent_id is required")
	}

	zap.L().Info("agent connected",
		zap.String("agentID", agentID),
		zap.String("clusterID", init.GetClusterID()),
		zap.String("version", init.GetVersion()),
	)

	// 2. 注册 agent，获取命令推送通道
	cmdChan, err := h.dataTunnelSvc.RegisterAgent(ctx, init)
	if err != nil {
		zap.L().Error("register agent failed", zap.String("agent_id", agentID), zap.Error(err))
		return status.Errorf(codes.Internal, "register agent failed: %v", err)
	}
	defer func() {
		if err := h.dataTunnelSvc.UnregisterAgent(context.Background(), agentID, clusterID, cmdChan); err != nil {
			zap.L().Warn("unregister agent", zap.String("agent_id", agentID), zap.Error(err))
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
				if err := h.dataTunnelSvc.HandleCommandResult(ctx, agentID, msg.GetTaskId(), result); err != nil {
					zap.L().Error("handle command result failed",
						zap.String("agent_id", agentID),
						zap.String("request_id", msg.GetTaskId()),
						zap.Int32("cmd", int32(result.GetCommandType())),
						zap.Error(err),
					)
				}
			}
		}
	}()

	// 4. 主循环：等待命令通道推送，实时下发
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
		case ac, ok := <-cmdChan:
			if !ok {
				// channel 被关闭：同一 agentID 的新副本已上线，本连接退出
				zap.L().Info("agent channel closed, another replica took over",
					zap.String("agent_id", agentID))
				return nil
			}
			msg := &pb.TunnelMessage{
				TaskId:  ac.RequestID,
				Payload: &pb.TunnelMessage_Command{Command: ac.Command},
			}
			if err := stream.Send(msg); err != nil {
				zap.L().Error("send command failed",
					zap.String("agent_id", agentID),
					zap.String("request_id", ac.RequestID),
					zap.Int32("cmd", int32(ac.Command.GetType())),
					zap.Error(err),
				)
				return err
			}
		}
	}
}
