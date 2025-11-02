package ioc

import (
	"github.com/serendipityConfusion/notification-platform/internal/pkg/config"
	"github.com/spf13/viper"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func InitEtcdClient() *clientv3.Client {
	cfg := clientv3.Config{}
	err := viper.UnmarshalKey("etcd", &cfg, viper.DecodeHook(viper.DecoderConfigOption(config.TagName("json"))))
	if err != nil {
		panic(err)
	}
	client, err := clientv3.New(cfg)
	if err != nil {
		panic(err)
	}
	return client
}
