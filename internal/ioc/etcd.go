package ioc

import (
	"time"

	"github.com/serendipityConfusion/notification-platform/internal/pkg/config"
	"github.com/spf13/viper"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func InitEtcdClient() *clientv3.Client {
	cfg := &config.EtcdConfig{}
	err := viper.UnmarshalKey("etcd", cfg, viper.DecodeHook(viper.DecoderConfigOption(config.TagName("yaml"))))
	if err != nil {
		panic(err)
	}

	// 设置默认值
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = 5 * time.Second
	}

	client, err := clientv3.New(clientv3.Config{
		Endpoints:   cfg.Endpoints,
		DialTimeout: cfg.DialTimeout,
		Username:    cfg.Username,
		Password:    cfg.Password,
	})
	if err != nil {
		panic(err)
	}
	return client
}
