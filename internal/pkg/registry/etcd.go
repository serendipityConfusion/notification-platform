package registry

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// EtcdRegistry 基于 etcd 的服务注册器
type EtcdRegistry struct {
	client      *clientv3.Client
	leaseID     clientv3.LeaseID
	keepAliveCh <-chan *clientv3.LeaseKeepAliveResponse
	mu          sync.RWMutex
	registered  map[string]*ServiceInfo // 已注册的服务
	closeOnce   sync.Once
	closeCh     chan struct{}
}

// EtcdConfig etcd 注册器配置
type EtcdConfig struct {
	Endpoints   []string      // etcd 端点列表
	DialTimeout time.Duration // 连接超时
	Username    string        // 认证用户名（可选）
	Password    string        // 认证密码（可选）
	Namespace   string        // 服务命名空间前缀，默认 "/services"
}

// NewEtcdRegistry 创建 etcd 服务注册器
func NewEtcdRegistry(client *clientv3.Client) *EtcdRegistry {
	return &EtcdRegistry{
		client:     client,
		registered: make(map[string]*ServiceInfo),
		closeCh:    make(chan struct{}),
	}
}

// NewEtcdRegistryWithConfig 通过配置创建 etcd 服务注册器
func NewEtcdRegistryWithConfig(cfg *EtcdConfig) (*EtcdRegistry, error) {
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
		return nil, fmt.Errorf("failed to create etcd client: %w", err)
	}

	return NewEtcdRegistry(client), nil
}

// Register 注册服务
func (r *EtcdRegistry) Register(ctx context.Context, info *ServiceInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 设置默认值
	if info.TTL == 0 {
		info.TTL = 10 * time.Second
	}
	if info.Namespace == "" {
		info.Namespace = "/services"
	}

	// 创建租约
	ttl := int64(info.TTL.Seconds())
	leaseResp, err := r.client.Grant(ctx, ttl)
	if err != nil {
		return fmt.Errorf("failed to grant lease: %w", err)
	}
	r.leaseID = leaseResp.ID

	// 构造服务 key
	serviceKey := r.buildServiceKey(info)

	// 注册服务到 etcd
	_, err = r.client.Put(ctx, serviceKey, info.Addr, clientv3.WithLease(r.leaseID))
	if err != nil {
		return fmt.Errorf("failed to register service: %w", err)
	}

	log.Printf("[Registry] Service registered: %s -> %s (lease: %d, ttl: %v)",
		serviceKey, info.Addr, r.leaseID, info.TTL)

	// 启动心跳保持
	keepAliveCh, err := r.client.KeepAlive(context.Background(), r.leaseID)
	if err != nil {
		return fmt.Errorf("failed to keep alive lease: %w", err)
	}
	r.keepAliveCh = keepAliveCh

	// 保存注册信息
	r.registered[info.Name] = info

	// 启动后台监听心跳
	go r.watchKeepAlive()

	return nil
}

// Deregister 注销服务
func (r *EtcdRegistry) Deregister(ctx context.Context, info *ServiceInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	serviceKey := r.buildServiceKey(info)

	// 从 etcd 删除服务
	_, err := r.client.Delete(ctx, serviceKey)
	if err != nil {
		return fmt.Errorf("failed to deregister service: %w", err)
	}

	log.Printf("[Registry] Service deregistered: %s", serviceKey)

	// 撤销租约
	if r.leaseID != 0 {
		_, err = r.client.Revoke(ctx, r.leaseID)
		if err != nil {
			log.Printf("[Registry] Failed to revoke lease: %v", err)
		}
		r.leaseID = 0
	}

	// 从注册列表中删除
	delete(r.registered, info.Name)

	return nil
}

// Close 关闭注册器
func (r *EtcdRegistry) Close() error {
	var err error
	r.closeOnce.Do(func() {
		close(r.closeCh)

		// 注销所有已注册的服务
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		r.mu.Lock()
		for _, info := range r.registered {
			if e := r.deregisterWithoutLock(ctx, info); e != nil {
				log.Printf("[Registry] Failed to deregister service %s: %v", info.Name, e)
			}
		}
		r.mu.Unlock()

		// 关闭 etcd 客户端
		if r.client != nil {
			err = r.client.Close()
		}

		log.Println("[Registry] Registry closed")
	})
	return err
}

// GetService 获取服务地址
func (r *EtcdRegistry) GetService(ctx context.Context, name string) (string, error) {
	key := fmt.Sprintf("/services/%s", name)
	resp, err := r.client.Get(ctx, key, clientv3.WithPrefix(), clientv3.WithLimit(1))
	if err != nil {
		return "", fmt.Errorf("failed to get service: %w", err)
	}

	if len(resp.Kvs) == 0 {
		return "", fmt.Errorf("service %s not found", name)
	}

	return string(resp.Kvs[0].Value), nil
}

// GetServiceList 获取服务的所有实例
func (r *EtcdRegistry) GetServiceList(ctx context.Context, name string) ([]string, error) {
	key := fmt.Sprintf("/services/%s", name)
	resp, err := r.client.Get(ctx, key, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("failed to get service list: %w", err)
	}

	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("service %s not found", name)
	}

	addresses := make([]string, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		addresses = append(addresses, string(kv.Value))
	}

	return addresses, nil
}

// Watch 监听服务变化
func (r *EtcdRegistry) Watch(ctx context.Context, name string) (<-chan Event, error) {
	key := fmt.Sprintf("/services/%s", name)
	watchCh := r.client.Watch(ctx, key, clientv3.WithPrefix())
	eventCh := make(chan Event, 10)

	go func() {
		defer close(eventCh)
		for {
			select {
			case <-ctx.Done():
				return
			case <-r.closeCh:
				return
			case wresp, ok := <-watchCh:
				if !ok {
					return
				}
				for _, ev := range wresp.Events {
					event := Event{
						Service: &ServiceInfo{
							Name: name,
							Addr: string(ev.Kv.Value),
						},
					}
					switch ev.Type {
					case clientv3.EventTypePut:
						if ev.IsCreate() {
							event.Type = EventTypeAdd
						} else {
							event.Type = EventTypeUpdate
						}
					case clientv3.EventTypeDelete:
						event.Type = EventTypeDelete
					}

					select {
					case eventCh <- event:
					case <-ctx.Done():
						return
					case <-r.closeCh:
						return
					}
				}
			}
		}
	}()

	return eventCh, nil
}

// watchKeepAlive 监听心跳续约
func (r *EtcdRegistry) watchKeepAlive() {
	for {
		select {
		case <-r.closeCh:
			return
		case ka, ok := <-r.keepAliveCh:
			if !ok {
				log.Println("[Registry] Keep-alive channel closed, service may be offline")
				return
			}
			if ka == nil {
				log.Println("[Registry] Keep-alive failed, lease may have expired")
				return
			}
			// 可以添加调试日志
			// log.Printf("[Registry] Keep-alive response: lease=%d, ttl=%d", ka.ID, ka.TTL)
		}
	}
}

// deregisterWithoutLock 注销服务（不加锁版本，内部使用）
func (r *EtcdRegistry) deregisterWithoutLock(ctx context.Context, info *ServiceInfo) error {
	serviceKey := r.buildServiceKey(info)
	_, err := r.client.Delete(ctx, serviceKey)
	if err != nil {
		return fmt.Errorf("failed to deregister service: %w", err)
	}
	return nil
}

// buildServiceKey 构造服务 key
func (r *EtcdRegistry) buildServiceKey(info *ServiceInfo) string {
	namespace := info.Namespace
	if namespace == "" {
		namespace = "/services"
	}
	return fmt.Sprintf("%s/%s", namespace, info.Name)
}

// 确保 EtcdRegistry 实现了 DiscoveryRegistry 接口
var _ DiscoveryRegistry = (*EtcdRegistry)(nil)
