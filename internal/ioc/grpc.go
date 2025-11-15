package ioc

import (
	"context"

	notificationpb "github.com/serendipityConfusion/notification-platform/api/gen/v1"
	grpcapi "github.com/serendipityConfusion/notification-platform/internal/api/grpc"
	"github.com/serendipityConfusion/notification-platform/internal/api/grpc/interceptor/log"
	"github.com/serendipityConfusion/notification-platform/internal/api/grpc/interceptor/metrics"
	"github.com/serendipityConfusion/notification-platform/internal/api/grpc/interceptor/tracing"
	"github.com/serendipityConfusion/notification-platform/internal/pkg/config"
	"github.com/spf13/viper"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
)

func InitGrpc(noserver *grpcapi.NotificationServer, etcdClient *clientv3.Client) *grpc.Server {
	conf := &config.GrpcConfig{}
	err := viper.UnmarshalKey("notification-server", conf, viper.DecodeHook(viper.DecoderConfigOption(config.TagName("yaml"))))
	if err != nil {
		panic(err)
	}
	_, eerr := etcdClient.Put(context.Background(), conf.Name, conf.Addr)
	if eerr != nil {
		panic(err)
	}
	// 创建observability拦截器
	metricsInterceptor := metrics.New().Build()
	logInterceptor := log.New().Build()
	// 拦截器定义
	traceInterceptor := tracing.UnaryServerInterceptor()
	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			metricsInterceptor,
			logInterceptor,
			traceInterceptor,
		),
	)
	//server.RegisterService(&notificationpb.NotificationService_ServiceDesc, noserver)
	notificationpb.RegisterNotificationServiceServer(server, noserver)
	notificationpb.RegisterNotificationQueryServiceServer(server, noserver)
	return server
}
