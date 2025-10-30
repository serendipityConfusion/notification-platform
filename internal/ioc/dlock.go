package ioc

import (
	"github.com/redis/go-redis/v9"
	"github.com/serendipityConfusion/notification-platform/internal/pkg/distribute_lock"
)

func InitDistributedLock(rdb *redis.Client) distribute_lock.Client {
	return distribute_lock.NewRedisDistributeClient(rdb)
}
