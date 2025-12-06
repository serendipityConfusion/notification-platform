package log

import (
	"go.uber.org/zap"
)

type LoggerInterface interface {
	Error(msg string, fields ...zap.Field)
	Info(msg string, fields ...zap.Field)
	Warn(msg string, fields ...zap.Field)
}

var _ LoggerInterface = (*Logger)(nil)

type Logger struct {
	*zap.Logger
}

func (l *Logger) Error(msg string, fields ...zap.Field) {
	l.Logger.Error(msg, fields...)
}

func (l *Logger) Warn(msg string, fields ...zap.Field) {
	l.Logger.Warn(msg, fields...)
}

func DefaultLogger() LoggerInterface {
	logger, _ := zap.NewProduction()
	return &Logger{Logger: logger}
}
