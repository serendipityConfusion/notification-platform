package ioc

import (
	"github.com/serendipityConfusion/notification-platform/internal/pkg/registry"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// InitRegistry 初始化服务注册器
// 使用已有的 etcd 客户端创建注册器
func InitRegistry(etcdClient *clientv3.Client) *registry.EtcdRegistry {
	return registry.NewEtcdRegistry(etcdClient)
}

// InitServiceInfo 初始化服务信息
// 返回 nil，将在 App.Run() 中从配置动态创建
func InitServiceInfo() *registry.ServiceInfo {
	// 返回 nil，让 App.Run() 从配置中动态创建服务信息
	// 也可以在这里预设一些默认值
	return nil
}
