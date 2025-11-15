package log

import (
	"context"
	"encoding/json"
	"github.com/serendipityConfusion/notification-platform/internal/pkg/log"
	"go.uber.org/zap"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Builder 日志拦截器构建器
type Builder struct {
	logger log.LoggerInterface
}

// New 创建日志拦截器构建器
func New() *Builder {
	return &Builder{
		logger: log.DefaultLogger(),
	}
}

// WithLogger 设置日志组件
func (b *Builder) WithLogger(logger log.LoggerInterface) *Builder {
	b.logger = logger
	return b
}

// Build 构建 gRPC 一元拦截器
func (b *Builder) Build() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// 记录开始时间
		startTime := time.Now()

		// 将请求对象转为 JSON 字符串进行记录
		reqJSON, _ := json.Marshal(req)
		b.logger.Info("gRPC request",
			zap.String("method", info.FullMethod),
			zap.String("request", string(reqJSON)),
			zap.Any("start_time", startTime))

		// 处理请求
		resp, err := handler(ctx, req)

		// 计算请求处理时间
		duration := time.Since(startTime)

		// 获取状态码
		st, _ := status.FromError(err)
		statusCode := st.Code()

		// 将响应对象转为 JSON 字符串进行记录
		respJSON, _ := json.Marshal(resp)

		if err != nil {
			// 如果有错误，记录错误日志
			b.logger.Error("gRPC response with error",
				zap.String("method", info.FullMethod),
				zap.String("status_code", statusCode.String()),
				zap.String("response", string(respJSON)),
				zap.Duration("duration", duration),
				zap.Any("error", err))
		} else {
			// 记录成功响应日志
			b.logger.Info("gRPC response",
				zap.String("method", info.FullMethod),
				zap.String("status_code", codes.OK.String()),
				zap.String("response", string(respJSON)),
				zap.Duration("duration", duration))
		}

		return resp, err
	}
}
