package interceptor

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"go.uber.org/zap"
)

// UnaryServerLogging 一元 RPC 日志拦截器
func UnaryServerLogging() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		var peerAddr string
		if p, ok := peer.FromContext(ctx); ok {
			peerAddr = p.Addr.String()
		}

		resp, err := handler(ctx, req)
		zap.L().Info("grpc unary",
			zap.String("method", info.FullMethod),
			zap.String("peer", peerAddr),
			zap.Duration("latency", time.Since(start)),
			zap.Error(err),
		)
		return resp, err
	}
}

// StreamServerLogging 流式 RPC 日志拦截器
func StreamServerLogging() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		var peerAddr string
		if p, ok := peer.FromContext(ss.Context()); ok {
			peerAddr = p.Addr.String()
		}

		err := handler(srv, ss)
		zap.L().Info("grpc stream",
			zap.String("method", info.FullMethod),
			zap.Bool("client_stream", info.IsClientStream),
			zap.Bool("server_stream", info.IsServerStream),
			zap.String("peer", peerAddr),
			zap.Duration("latency", time.Since(start)),
			zap.Error(err),
		)
		return err
	}
}
