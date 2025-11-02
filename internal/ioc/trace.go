package ioc

import (
	"context"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
)

func InitJeagerTracer() *trace.TracerProvider {
	// 创建资源信息
	res, err := newResource()
	if err != nil {
		panic(err)
	}

	// 初始化传播器
	otel.SetTextMapPropagator(newPropagator())

	// 初始化 tracer provider
	tp, err := newTracerProvider(res)
	if err != nil {
		panic(err)
	}

	// 设置全局 tracer provider
	otel.SetTracerProvider(tp)

	return tp
}

// newResource 创建 OpenTelemetry 资源
func newResource() (*resource.Resource, error) {
	serviceName := viper.GetString("trace.jeager.serviceName")
	serviceVersion := "v0.0.1"

	return resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		),
	)
}

// newTracerProvider 创建 tracer provider
func newTracerProvider(res *resource.Resource) (*trace.TracerProvider, error) {
	// 从配置读取 jeager 端点地址
	jeagerEndpoint := viper.GetString("trace.jeager.endpoint")
	exporter, err := otlptracehttp.New(context.Background(), otlptracehttp.WithEndpoint(jeagerEndpoint))
	if err != nil {
		panic(err)
	}
	// 创建 tracer provider
	return trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(res),
	), nil
}

// newPropagator 创建上下文传播器
func newPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}
