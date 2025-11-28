package discovery

import (
	"context"
	"fmt"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ServiceDiscovery 服务发现客户端
type ServiceDiscovery struct {
	client *clientv3.Client
	prefix string // 服务注册的前缀，默认为 /services/
	mu     sync.RWMutex
	// 缓存服务地址列表
	serviceCache map[string][]string
}

// NewServiceDiscovery 创建服务发现客户端
func NewServiceDiscovery(client *clientv3.Client) *ServiceDiscovery {
	return &ServiceDiscovery{
		client:       client,
		prefix:       "/services/",
		serviceCache: make(map[string][]string),
	}
}

// GetService 获取指定服务的地址（返回第一个可用的）
func (sd *ServiceDiscovery) GetService(ctx context.Context, serviceName string) (string, error) {
	key := sd.prefix + serviceName
	resp, err := sd.client.Get(ctx, key, clientv3.WithPrefix())
	if err != nil {
		return "", fmt.Errorf("failed to get service from etcd: %w", err)
	}

	if len(resp.Kvs) == 0 {
		return "", fmt.Errorf("service %s not found", serviceName)
	}

	// 返回第一个服务地址
	return string(resp.Kvs[0].Value), nil
}

// GetServiceList 获取指定服务的所有实例地址
func (sd *ServiceDiscovery) GetServiceList(ctx context.Context, serviceName string) ([]string, error) {
	key := sd.prefix + serviceName
	resp, err := sd.client.Get(ctx, key, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("failed to get service list from etcd: %w", err)
	}

	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("service %s not found", serviceName)
	}

	addresses := make([]string, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		addresses = append(addresses, string(kv.Value))
	}

	return addresses, nil
}

// WatchService 监听服务变化
func (sd *ServiceDiscovery) WatchService(ctx context.Context, serviceName string, callback func(EventType, string)) {
	key := sd.prefix + serviceName
	watchChan := sd.client.Watch(ctx, key, clientv3.WithPrefix())

	for wresp := range watchChan {
		for _, ev := range wresp.Events {
			eventType := EventTypeUnknown
			switch ev.Type {
			case clientv3.EventTypePut:
				eventType = EventTypeAdd
			case clientv3.EventTypeDelete:
				eventType = EventTypeDelete
			}
			callback(eventType, string(ev.Kv.Value))
		}
	}
}

// GetAllServices 获取所有注册的服务
func (sd *ServiceDiscovery) GetAllServices(ctx context.Context) (map[string][]string, error) {
	resp, err := sd.client.Get(ctx, sd.prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("failed to get all services from etcd: %w", err)
	}

	services := make(map[string][]string)
	for _, kv := range resp.Kvs {
		// 提取服务名称（去掉前缀）
		key := string(kv.Key)
		serviceName := key[len(sd.prefix):]
		addr := string(kv.Value)

		services[serviceName] = append(services[serviceName], addr)
	}

	return services, nil
}

// DialService 创建到指定服务的 gRPC 连接
func (sd *ServiceDiscovery) DialService(ctx context.Context, serviceName string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	addr, err := sd.GetService(ctx, serviceName)
	if err != nil {
		return nil, err
	}

	// 默认使用不安全的连接（开发环境）
	if len(opts) == 0 {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.DialContext(ctx, addr, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to dial service %s at %s: %w", serviceName, addr, err)
	}

	return conn, nil
}

// StartWatch 启动服务监听，持续更新缓存
func (sd *ServiceDiscovery) StartWatch(ctx context.Context) {
	// 初始化缓存
	services, err := sd.GetAllServices(ctx)
	if err == nil {
		sd.mu.Lock()
		sd.serviceCache = services
		sd.mu.Unlock()
	}

	// 监听所有服务变化
	go func() {
		watchChan := sd.client.Watch(ctx, sd.prefix, clientv3.WithPrefix())
		for wresp := range watchChan {
			sd.mu.Lock()
			for _, ev := range wresp.Events {
				key := string(ev.Kv.Key)
				serviceName := key[len(sd.prefix):]
				addr := string(ev.Kv.Value)

				switch ev.Type {
				case clientv3.EventTypePut:
					// 添加或更新服务
					sd.serviceCache[serviceName] = append(sd.serviceCache[serviceName], addr)
				case clientv3.EventTypeDelete:
					// 删除服务
					addrs := sd.serviceCache[serviceName]
					for i, a := range addrs {
						if a == addr {
							sd.serviceCache[serviceName] = append(addrs[:i], addrs[i+1:]...)
							break
						}
					}
					// 如果服务没有实例了，删除该服务
					if len(sd.serviceCache[serviceName]) == 0 {
						delete(sd.serviceCache, serviceName)
					}
				}
			}
			sd.mu.Unlock()
		}
	}()
}

// GetCachedService 从缓存中获取服务地址（需要先调用 StartWatch）
func (sd *ServiceDiscovery) GetCachedService(serviceName string) (string, error) {
	sd.mu.RLock()
	defer sd.mu.RUnlock()

	addrs, exists := sd.serviceCache[serviceName]
	if !exists || len(addrs) == 0 {
		return "", fmt.Errorf("service %s not found in cache", serviceName)
	}

	// 简单的轮询策略：返回第一个
	return addrs[0], nil
}

// GetCachedServiceList 从缓存中获取服务列表（需要先调用 StartWatch）
func (sd *ServiceDiscovery) GetCachedServiceList(serviceName string) ([]string, error) {
	sd.mu.RLock()
	defer sd.mu.RUnlock()

	addrs, exists := sd.serviceCache[serviceName]
	if !exists || len(addrs) == 0 {
		return nil, fmt.Errorf("service %s not found in cache", serviceName)
	}

	// 返回副本，避免外部修改
	result := make([]string, len(addrs))
	copy(result, addrs)
	return result, nil
}

// WaitForService 等待服务上线（带超时）
func (sd *ServiceDiscovery) WaitForService(ctx context.Context, serviceName string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 先尝试获取一次
	addr, err := sd.GetService(ctx, serviceName)
	if err == nil {
		return addr, nil
	}

	// 如果没有，则监听服务上线
	key := sd.prefix + serviceName
	watchChan := sd.client.Watch(ctx, key, clientv3.WithPrefix())

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("timeout waiting for service %s", serviceName)
		case wresp := <-watchChan:
			for _, ev := range wresp.Events {
				if ev.Type == clientv3.EventTypePut {
					return string(ev.Kv.Value), nil
				}
			}
		}
	}
}

// EventType 服务事件类型
type EventType int

const (
	EventTypeUnknown EventType = iota
	EventTypeAdd               // 服务添加
	EventTypeDelete            // 服务删除
)

func (e EventType) String() string {
	switch e {
	case EventTypeAdd:
		return "Add"
	case EventTypeDelete:
		return "Delete"
	default:
		return "Unknown"
	}
}
