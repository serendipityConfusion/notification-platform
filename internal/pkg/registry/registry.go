package registry

import (
	"context"
	"time"
)

// ServiceInfo 服务信息
type ServiceInfo struct {
	Name      string            // 服务名称
	Addr      string            // 服务地址
	Metadata  map[string]string // 元数据（可选）
	TTL       time.Duration     // 心跳间隔（用于健康检查）
	Namespace string            // 命名空间（可选，用于服务隔离）
}

// Registry 服务注册接口
type Registry interface {
	// Register 注册服务到注册中心
	// 该方法会启动后台心跳保持服务在线状态
	Register(ctx context.Context, info *ServiceInfo) error

	// Deregister 从注册中心注销服务
	Deregister(ctx context.Context, info *ServiceInfo) error

	// Close 关闭注册器，清理资源
	Close() error
}

// DiscoveryRegistry 扩展接口，支持服务发现
type DiscoveryRegistry interface {
	Registry

	// GetService 获取服务地址
	GetService(ctx context.Context, name string) (string, error)

	// GetServiceList 获取服务的所有实例地址
	GetServiceList(ctx context.Context, name string) ([]string, error)

	// Watch 监听服务变化
	Watch(ctx context.Context, name string) (<-chan Event, error)
}

// Event 服务变化事件
type Event struct {
	Type    EventType    // 事件类型
	Service *ServiceInfo // 服务信息
}

// EventType 事件类型
type EventType int

const (
	EventTypeAdd    EventType = iota + 1 // 服务添加
	EventTypeUpdate                      // 服务更新
	EventTypeDelete                      // 服务删除
)

func (e EventType) String() string {
	switch e {
	case EventTypeAdd:
		return "Add"
	case EventTypeUpdate:
		return "Update"
	case EventTypeDelete:
		return "Delete"
	default:
		return "Unknown"
	}
}
