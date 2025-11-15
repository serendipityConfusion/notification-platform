//go:build wireinject

package ioc

import (
	"github.com/google/wire"
	grpcapi "github.com/serendipityConfusion/notification-platform/internal/api/grpc"
	"github.com/serendipityConfusion/notification-platform/internal/ioc"
	"github.com/serendipityConfusion/notification-platform/internal/repository"
	"github.com/serendipityConfusion/notification-platform/internal/repository/cache/redis"
	"github.com/serendipityConfusion/notification-platform/internal/repository/dao"
	"github.com/serendipityConfusion/notification-platform/internal/service"
)

var (
	BaseSet = wire.NewSet(
		ioc.InitDB,
		ioc.InitRedis,
		ioc.InitIDGenerator,
		ioc.InitDistributedLock,
		ioc.InitEtcdClient,
		ioc.InitJeagerTracer,
	)

	notificationSvcSet = wire.NewSet(
		service.NewNotificationService,
		repository.NewNotificationRepository,
		dao.NewNotificationDAO,
		redis.NewQuotaCache,
	)
)

func InitGrpcServer() *ioc.App {
	wire.Build(
		BaseSet,
		notificationSvcSet,
		grpcapi.NewServer,
		ioc.InitGrpc,
		wire.Struct(new(ioc.App), "*"),
	)
	return &ioc.App{}
}
