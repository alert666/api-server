package server

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"time"

	pb "github.com/alert666/alertmanager-proto/gen/go/data_tunnel/v1"
	"github.com/alert666/api-server/base/conf"
	"github.com/alert666/api-server/grpc/handler"
	"github.com/alert666/api-server/grpc/interceptor"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

// GRPCServer 实现 base/server.ServerInterface，由 Application 统一管理生命周期
type GRPCServer struct {
	server   *grpc.Server
	addr     string
	listener net.Listener
}

// NewGRPCServer 创建 gRPC server
func NewGRPCServer(addr string, tunnelHandler *handler.TunnelHandler) (*GRPCServer, error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("grpc listen %s failed: %w", addr, err)
	}

	var serverOpts []grpc.ServerOption

	// 加载 TLS / mTLS
	certFile := conf.GetGrpcTLSCertFile()
	keyFile := conf.GetGrpcTLSKeyFile()
	if certFile != "" && keyFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load gRPC TLS cert: %w", err)
		}

		tlsCfg := &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}

		if caFile := conf.GetGrpcTLSCAFile(); caFile != "" {
			caPEM, err := os.ReadFile(caFile)
			if err != nil {
				return nil, fmt.Errorf("failed to read CA cert: %w", err)
			}
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(caPEM) {
				return nil, fmt.Errorf("failed to parse CA cert: %s", caFile)
			}
			tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
			tlsCfg.ClientCAs = pool
			zap.L().Info("gRPC mTLS enabled (client cert required)",
				zap.String("certFile", certFile), zap.String("caFile", caFile))
		} else {
			zap.L().Info("gRPC TLS enabled", zap.String("certFile", certFile))
		}

		serverOpts = append(serverOpts, grpc.Creds(credentials.NewTLS(tlsCfg)))
	} else {
		zap.L().Warn("gRPC TLS not configured, using insecure connection")
	}

	// 连接保活：定期 ping 检测客户端是否存活
	serverOpts = append(serverOpts,
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             10 * time.Second, // 客户端 ping 最小间隔
			PermitWithoutStream: true,             // 允许无活动流时也发 ping
		}),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    30 * time.Second, // 服务端 ping 间隔
			Timeout: 10 * time.Second, // 超时时间，超时则断开连接
		}),
	)

	serverOpts = append(serverOpts,
		grpc.ChainUnaryInterceptor(
			interceptor.UnaryServerLogging(),
		),
		grpc.ChainStreamInterceptor(
			interceptor.StreamServerLogging(),
		),
	)

	srv := grpc.NewServer(serverOpts...)

	pb.RegisterTunnelServiceServer(srv, tunnelHandler)

	return &GRPCServer{
		server:   srv,
		addr:     addr,
		listener: listener,
	}, nil
}

func (s *GRPCServer) Start() error {
	zap.L().Info("grpc server listening", zap.String("addr", s.addr))
	return s.server.Serve(s.listener)
}

func (s *GRPCServer) Stop() error {
	zap.L().Info("grpc server stopping...")
	s.server.Stop()
	zap.L().Info("grpc server stopped")
	return nil
}
