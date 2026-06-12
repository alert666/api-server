package server

import (
	"fmt"
	"net"

	"github.com/alert666/api-server/grpc/handler"
	"github.com/alert666/api-server/grpc/interceptor"
	pb "github.com/alert666/alertmanager-proto/gen/go/data_tunnel/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// GRPCServer 实现 base/server.ServerInterface，由 Application 统一管理生命周期
type GRPCServer struct {
	server   *grpc.Server
	addr     string
	listener net.Listener
}

// NewGRPCServer 创建 gRPC server
// addr: 监听地址，如 ":9090"
func NewGRPCServer(addr string, tunnelHandler *handler.TunnelHandler) (*GRPCServer, error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("grpc listen %s failed: %w", addr, err)
	}

	srv := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			interceptor.UnaryServerLogging(),
		),
		grpc.ChainStreamInterceptor(
			interceptor.StreamServerLogging(),
		),
	)

	// 注册所有 gRPC service
	pb.RegisterTunnelServiceServer(srv, tunnelHandler)

	return &GRPCServer{
		server:   srv,
		addr:     addr,
		listener: listener,
	}, nil
}

// Start 启动 gRPC server，阻塞直到 Stop 被调用
func (s *GRPCServer) Start() error {
	zap.S().Infof("grpc server listening on %s", s.addr)
	return s.server.Serve(s.listener)
}

// Stop 优雅关闭 gRPC server
func (s *GRPCServer) Stop() error {
	zap.L().Info("grpc server stopping...")
	s.server.GracefulStop()
	zap.L().Info("grpc server stopped")
	return nil
}
