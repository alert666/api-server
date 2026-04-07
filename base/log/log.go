package log

import (
	"context"
	golog "log"
	"os"
	"time"

	"github.com/qinquanliuxiang666/alertmanager/base/conf"
	"github.com/qinquanliuxiang666/alertmanager/base/constant"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewLogger() {
	var (
		encoder  zapcore.Encoder
		writer   zapcore.WriteSyncer
		logLevel zapcore.Level
	)
	logLevelStr := conf.GetLogLevel()
	timeZone := conf.GetServerTimeZone()
	logEncoder := conf.GetLogEncoder()
	cst, err := time.LoadLocation(timeZone)
	if err != nil {
		golog.Printf("failed to load location %s: %v, use local time instead", timeZone, err)
		cst = time.Local
	}

	config := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		EncodeDuration: zapcore.SecondsDurationEncoder,
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeTime: func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendString(t.In(cst).Format(time.RFC3339))
		},
		EncodeCaller: zapcore.ShortCallerEncoder,
	}

	switch logEncoder {
	case "json":
		encoder = zapcore.NewJSONEncoder(config)
	case "console":
		config.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(config)
	default:
		config.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(config)
	}

	writer = zapcore.AddSync(os.Stdout)

	switch logLevelStr {
	case "debug":
		logLevel = zap.DebugLevel
	case "info":
		logLevel = zap.InfoLevel
	default:
		logLevel = zap.InfoLevel
	}
	core := zapcore.NewCore(encoder, writer, logLevel)
	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zap.FatalLevel))
	zap.ReplaceGlobals(logger)
	zap.L().Info("log initialization successful", zap.String("level", logLevelStr))
}

func WithRequestID(ctx context.Context) *zap.Logger {
	return zap.L().With(zap.String("request-id", getRequestIDFromContext(ctx)))
}

func WithBody(ctx context.Context, body any) *zap.Logger {
	return WithRequestID(ctx).With(zap.Any("body", body))
}

// getRequestIDFromContext 从上下文中获取请求 ID
func getRequestIDFromContext(ctx context.Context) string {
	if reqID, ok := ctx.Value(constant.RequestIDContextKey).(string); ok {
		return reqID
	}
	return ""
}
