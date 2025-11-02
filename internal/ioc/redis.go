package ioc

import (
	"github.com/redis/go-redis/v9"
	"github.com/serendipityConfusion/notification-platform/internal/pkg/config"
	"github.com/serendipityConfusion/notification-platform/internal/pkg/redis/metrics"
	"github.com/serendipityConfusion/notification-platform/internal/pkg/redis/tracing"
	"github.com/spf13/viper"
)

func InitRedis() *redis.Client {
	conf := config.RedisConfig{}
	err := viper.UnmarshalKey("redis", &conf, viper.DecodeHook(viper.DecoderConfigOption(config.TagName("yaml"))))
	if err != nil {
		panic(err)
	}
	client := redis.NewClient(&redis.Options{
		Addr:     conf.Addr,
		Password: conf.Password,
		Username: conf.UserName,
	})
	client = tracing.WithTracing(client)
	client = metrics.WithMetrics(client)
	return client
}
