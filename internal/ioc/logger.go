package ioc

import (
	"github.com/serendipityConfusion/notification-platform/internal/pkg/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// InitLogger 初始化日志记录器
func InitLogger() log.LoggerInterface {
	// 根据环境配置日志级别
	// 开发环境使用 Development 配置，生产环境使用 Production 配置
	config := zap.NewProductionConfig()

	// 配置日志编码
	config.Encoding = "json"

	// 配置日志级别
	config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)

	// 配置输出路径
	config.OutputPaths = []string{"stdout"}
	config.ErrorOutputPaths = []string{"stderr"}

	// 配置日志字段
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncoderConfig.MessageKey = "message"
	config.EncoderConfig.LevelKey = "level"
	config.EncoderConfig.CallerKey = "caller"
	config.EncoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder

	// 构建 logger
	logger, err := config.Build(
		zap.AddCaller(),
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	if err != nil {
		// 如果构建失败，使用默认 logger
		return log.DefaultLogger()
	}

	return &log.Logger{Logger: logger}
}

// InitDevelopmentLogger 初始化开发环境日志记录器
func InitDevelopmentLogger() log.LoggerInterface {
	config := zap.NewDevelopmentConfig()

	// 开发环境使用 console 编码，便于阅读
	config.Encoding = "console"
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := config.Build(
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.WarnLevel),
	)
	if err != nil {
		return log.DefaultLogger()
	}

	return &log.Logger{Logger: logger}
}
