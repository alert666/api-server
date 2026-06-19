package feishu

import (
	"context"
	"fmt"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"go.uber.org/zap"
)

// zapLoggerAdapter 实现 larkcore.Logger 接口，将飞书 SDK 的日志桥接到 zap，输出格式跟随 log.encoder 配置（JSON/console）。
type zapLoggerAdapter struct {
	logger *zap.Logger
}

func newZapLoggerAdapter(logger *zap.Logger) *zapLoggerAdapter {
	return &zapLoggerAdapter{logger: logger}
}

var _ larkcore.Logger = (*zapLoggerAdapter)(nil)

func (l *zapLoggerAdapter) formatMsg(args ...interface{}) string {
	return fmt.Sprint(args...)
}

func (l *zapLoggerAdapter) Debug(ctx context.Context, args ...interface{}) {
	l.logger.Debug(l.formatMsg(args...))
}

func (l *zapLoggerAdapter) Info(ctx context.Context, args ...interface{}) {
	l.logger.Info(l.formatMsg(args...))
}

func (l *zapLoggerAdapter) Warn(ctx context.Context, args ...interface{}) {
	l.logger.Warn(l.formatMsg(args...))
}

func (l *zapLoggerAdapter) Error(ctx context.Context, args ...interface{}) {
	l.logger.Error(l.formatMsg(args...))
}
