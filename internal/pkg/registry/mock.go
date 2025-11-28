package registry

import (
	"context"
	"fmt"
	"sync"
)

// MockRegistry Mock 服务注册器，用于测试
type MockRegistry struct {
	mu sync.RWMutex

	// 可自定义的方法实现
	RegisterFunc       func(ctx context.Context, info *ServiceInfo) error
	DeregisterFunc     func(ctx context.Context, info *ServiceInfo) error
	CloseFunc          func() error
	GetServiceFunc     func(ctx context.Context, name string) (string, error)
	GetServiceListFunc func(ctx context.Context, name string) ([]string, error)
	WatchFunc          func(ctx context.Context, name string) (<-chan Event, error)

	// 记录调用信息
	RegisterCalls       []*ServiceInfo
	DeregisterCalls     []*ServiceInfo
	CloseCalls          int
	GetServiceCalls     []string
	GetServiceListCalls []string
	WatchCalls          []string

	// 存储已注册的服务
	services map[string]*ServiceInfo
}

// NewMockRegistry 创建 Mock 注册器
func NewMockRegistry() *MockRegistry {
	return &MockRegistry{
		services: make(map[string]*ServiceInfo),
	}
}

// Register 注册服务
func (m *MockRegistry) Register(ctx context.Context, info *ServiceInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.RegisterCalls = append(m.RegisterCalls, info)

	if m.RegisterFunc != nil {
		return m.RegisterFunc(ctx, info)
	}

	// 默认行为：存储服务信息
	m.services[info.Name] = info
	return nil
}

// Deregister 注销服务
func (m *MockRegistry) Deregister(ctx context.Context, info *ServiceInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.DeregisterCalls = append(m.DeregisterCalls, info)

	if m.DeregisterFunc != nil {
		return m.DeregisterFunc(ctx, info)
	}

	// 默认行为：删除服务信息
	delete(m.services, info.Name)
	return nil
}

// Close 关闭注册器
func (m *MockRegistry) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CloseCalls++

	if m.CloseFunc != nil {
		return m.CloseFunc()
	}

	// 默认行为：清空服务列表
	m.services = make(map[string]*ServiceInfo)
	return nil
}

// GetService 获取服务地址
func (m *MockRegistry) GetService(ctx context.Context, name string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.GetServiceCalls = append(m.GetServiceCalls, name)

	if m.GetServiceFunc != nil {
		return m.GetServiceFunc(ctx, name)
	}

	// 默认行为：从存储中获取
	if info, exists := m.services[name]; exists {
		return info.Addr, nil
	}

	return "", fmt.Errorf("service %s not found", name)
}

// GetServiceList 获取服务的所有实例地址
func (m *MockRegistry) GetServiceList(ctx context.Context, name string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.GetServiceListCalls = append(m.GetServiceListCalls, name)

	if m.GetServiceListFunc != nil {
		return m.GetServiceListFunc(ctx, name)
	}

	// 默认行为：返回单个服务地址
	if info, exists := m.services[name]; exists {
		return []string{info.Addr}, nil
	}

	return nil, fmt.Errorf("service %s not found", name)
}

// Watch 监听服务变化
func (m *MockRegistry) Watch(ctx context.Context, name string) (<-chan Event, error) {
	m.mu.Lock()
	m.WatchCalls = append(m.WatchCalls, name)
	m.mu.Unlock()

	if m.WatchFunc != nil {
		return m.WatchFunc(ctx, name)
	}

	// 默认行为：返回一个空的 channel
	ch := make(chan Event)
	go func() {
		<-ctx.Done()
		close(ch)
	}()
	return ch, nil
}

// GetRegisteredService 获取已注册的服务信息（测试辅助方法）
func (m *MockRegistry) GetRegisteredService(name string) (*ServiceInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	info, exists := m.services[name]
	return info, exists
}

// GetAllRegisteredServices 获取所有已注册的服务（测试辅助方法）
func (m *MockRegistry) GetAllRegisteredServices() map[string]*ServiceInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*ServiceInfo, len(m.services))
	for k, v := range m.services {
		result[k] = v
	}
	return result
}

// Reset 重置 Mock 状态（测试辅助方法）
func (m *MockRegistry) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.RegisterCalls = nil
	m.DeregisterCalls = nil
	m.CloseCalls = 0
	m.GetServiceCalls = nil
	m.GetServiceListCalls = nil
	m.WatchCalls = nil
	m.services = make(map[string]*ServiceInfo)
}

// 确保 MockRegistry 实现了接口
var _ Registry = (*MockRegistry)(nil)
var _ DiscoveryRegistry = (*MockRegistry)(nil)

// MockRegistryBuilder Mock 注册器构建器，提供流畅的 API
type MockRegistryBuilder struct {
	mock *MockRegistry
}

// NewMockRegistryBuilder 创建 Mock 注册器构建器
func NewMockRegistryBuilder() *MockRegistryBuilder {
	return &MockRegistryBuilder{
		mock: NewMockRegistry(),
	}
}

// WithRegisterFunc 设置 Register 方法的行为
func (b *MockRegistryBuilder) WithRegisterFunc(fn func(ctx context.Context, info *ServiceInfo) error) *MockRegistryBuilder {
	b.mock.RegisterFunc = fn
	return b
}

// WithDeregisterFunc 设置 Deregister 方法的行为
func (b *MockRegistryBuilder) WithDeregisterFunc(fn func(ctx context.Context, info *ServiceInfo) error) *MockRegistryBuilder {
	b.mock.DeregisterFunc = fn
	return b
}

// WithGetServiceFunc 设置 GetService 方法的行为
func (b *MockRegistryBuilder) WithGetServiceFunc(fn func(ctx context.Context, name string) (string, error)) *MockRegistryBuilder {
	b.mock.GetServiceFunc = fn
	return b
}

// WithPreRegisteredServices 预注册一些服务
func (b *MockRegistryBuilder) WithPreRegisteredServices(services ...*ServiceInfo) *MockRegistryBuilder {
	for _, svc := range services {
		b.mock.services[svc.Name] = svc
	}
	return b
}

// Build 构建 Mock 注册器
func (b *MockRegistryBuilder) Build() *MockRegistry {
	return b.mock
}
