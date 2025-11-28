# 使用示例和测试文档

本文档提供了优化后架构的详细使用示例，包括基本使用、测试、扩展等场景。

## 目录

- [基本使用](#基本使用)
- [单元测试](#单元测试)
- [集成测试](#集成测试)
- [自定义实现](#自定义实现)
- [高级用法](#高级用法)

## 基本使用

### 1. 启动应用（完整示例）

```go
// cmd/platform/main.go
package main

import (
    "log"
    "github.com/serendipityConfusion/notification-platform/cmd/platform/ioc"
    "github.com/serendipityConfusion/notification-platform/internal/pkg/config"
)

func main() {
    // 1. 初始化配置
    if err := config.InitViperConfig(); err != nil {
        log.Fatalf("Failed to init config: %v", err)
    }

    // 2. 使用 Wire 自动注入依赖
    app := ioc.InitGrpcServer()

    // 3. 运行应用（自动注册服务、启动服务器）
    if err := app.Run(); err != nil {
        log.Fatalf("Application error: %v", err)
    }
}
```

### 2. 使用 Registry 接口（独立使用）

```go
package main

import (
    "context"
    "log"
    "time"
    
    "github.com/serendipityConfusion/notification-platform/internal/pkg/registry"
    clientv3 "go.etcd.io/etcd/client/v3"
)

func main() {
    // 创建 etcd 客户端
    client, err := clientv3.New(clientv3.Config{
        Endpoints:   []string{"localhost:2379"},
        DialTimeout: 5 * time.Second,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // 创建服务注册器
    reg := registry.NewEtcdRegistry(client)
    defer reg.Close()

    // 注册服务
    serviceInfo := &registry.ServiceInfo{
        Name:      "my-service",
        Addr:      "localhost:8080",
        TTL:       10 * time.Second,
        Namespace: "/services",
        Metadata: map[string]string{
            "version": "1.0.0",
            "region":  "us-west",
        },
    }

    ctx := context.Background()
    if err := reg.Register(ctx, serviceInfo); err != nil {
        log.Fatalf("Failed to register: %v", err)
    }

    log.Println("Service registered successfully")

    // 保持服务运行...
    time.Sleep(30 * time.Second)

    // 注销服务
    if err := reg.Deregister(ctx, serviceInfo); err != nil {
        log.Printf("Failed to deregister: %v", err)
    }
}
```

### 3. 使用 ConfigLoader 接口

```go
package main

import (
    "log"
    "github.com/serendipityConfusion/notification-platform/internal/pkg/config"
)

type MyConfig struct {
    Host     string `yaml:"host"`
    Port     int    `yaml:"port"`
    Timeout  int    `yaml:"timeout"`
    Debug    bool   `yaml:"debug"`
}

func main() {
    // 初始化配置
    if err := config.InitViperConfig(); err != nil {
        log.Fatal(err)
    }

    // 创建配置加载器
    loader := config.NewViperConfigLoader()

    // 加载配置到结构体
    var cfg MyConfig
    if err := loader.Load("my-service", &cfg); err != nil {
        log.Fatal(err)
    }

    log.Printf("Config loaded: %+v", cfg)

    // 或者直接获取单个值
    host := loader.GetString("my-service.host")
    port := loader.GetInt("my-service.port")
    debug := loader.GetBool("my-service.debug")

    log.Printf("Host: %s, Port: %d, Debug: %v", host, port, debug)
}
```

## 单元测试

### 1. 测试使用 Mock Registry

```go
// internal/ioc/app_test.go
package ioc_test

import (
    "context"
    "testing"
    "time"

    "github.com/serendipityConfusion/notification-platform/internal/ioc"
    "github.com/serendipityConfusion/notification-platform/internal/pkg/config"
    "github.com/serendipityConfusion/notification-platform/internal/pkg/registry"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "google.golang.org/grpc"
)

func TestApp_Register(t *testing.T) {
    // 创建 Mock Registry
    mockReg := registry.NewMockRegistry()
    
    // 创建 Mock ConfigLoader
    mockLoader := &MockConfigLoader{
        data: map[string]interface{}{
            "notification-server": &config.GrpcConfig{
                Addr: "localhost:8080",
                Name: "test-service",
            },
        },
    }

    // 创建 App
    app := &ioc.App{
        GrpcServer:   grpc.NewServer(),
        Registry:     mockReg,
        ConfigLoader: mockLoader,
    }

    // 在 goroutine 中运行（因为 Run 会阻塞）
    go func() {
        _ = app.Run()
    }()

    // 等待一下让服务注册
    time.Sleep(100 * time.Millisecond)

    // 验证服务已注册
    assert.Len(t, mockReg.RegisterCalls, 1, "Should register once")
    
    registeredInfo := mockReg.RegisterCalls[0]
    assert.Equal(t, "test-service", registeredInfo.Name)
    assert.Equal(t, "localhost:8080", registeredInfo.Addr)

    // 验证服务在注册表中
    info, exists := mockReg.GetRegisteredService("test-service")
    require.True(t, exists, "Service should exist in registry")
    assert.Equal(t, "localhost:8080", info.Addr)
}

func TestApp_Deregister(t *testing.T) {
    mockReg := registry.NewMockRegistry()
    
    // 预先注册一个服务
    serviceInfo := &registry.ServiceInfo{
        Name: "test-service",
        Addr: "localhost:8080",
    }
    mockReg.Register(context.Background(), serviceInfo)

    app := &ioc.App{
        Registry:    mockReg,
        ServiceInfo: serviceInfo,
    }

    // 调用 shutdown
    err := app.shutdown()
    assert.NoError(t, err)

    // 验证服务已注销
    assert.Len(t, mockReg.DeregisterCalls, 1, "Should deregister once")
    assert.Equal(t, 1, mockReg.CloseCalls, "Should close once")
}

// MockConfigLoader 实现
type MockConfigLoader struct {
    data map[string]interface{}
}

func (m *MockConfigLoader) Load(key string, target interface{}) error {
    if val, ok := m.data[key]; ok {
        // 简单的类型断言赋值
        switch t := target.(type) {
        case *config.GrpcConfig:
            *t = *val.(*config.GrpcConfig)
        }
    }
    return nil
}

func (m *MockConfigLoader) GetString(key string) string   { return "" }
func (m *MockConfigLoader) GetInt(key string) int         { return 0 }
func (m *MockConfigLoader) GetBool(key string) bool       { return false }
func (m *MockConfigLoader) GetDuration(key string) time.Duration { return 0 }
```

### 2. 测试 Registry 行为

```go
// internal/pkg/registry/etcd_test.go
package registry_test

import (
    "context"
    "testing"
    "time"

    "github.com/serendipityConfusion/notification-platform/internal/pkg/registry"
    "github.com/stretchr/testify/assert"
)

func TestMockRegistry_Register(t *testing.T) {
    // 使用构建器创建 Mock
    mock := registry.NewMockRegistryBuilder().
        WithRegisterFunc(func(ctx context.Context, info *registry.ServiceInfo) error {
            // 自定义注册逻辑
            assert.Equal(t, "my-service", info.Name)
            return nil
        }).
        Build()

    info := &registry.ServiceInfo{
        Name: "my-service",
        Addr: "localhost:8080",
    }

    err := mock.Register(context.Background(), info)
    assert.NoError(t, err)
    assert.Len(t, mock.RegisterCalls, 1)
}

func TestMockRegistry_GetService(t *testing.T) {
    // 预注册一些服务
    mock := registry.NewMockRegistryBuilder().
        WithPreRegisteredServices(
            &registry.ServiceInfo{Name: "service-1", Addr: "localhost:8080"},
            &registry.ServiceInfo{Name: "service-2", Addr: "localhost:8081"},
        ).
        Build()

    // 测试获取服务
    addr, err := mock.GetService(context.Background(), "service-1")
    assert.NoError(t, err)
    assert.Equal(t, "localhost:8080", addr)

    // 测试获取不存在的服务
    _, err = mock.GetService(context.Background(), "non-existent")
    assert.Error(t, err)
}

func TestMockRegistry_CallTracking(t *testing.T) {
    mock := registry.NewMockRegistry()

    // 执行一些操作
    ctx := context.Background()
    info := &registry.ServiceInfo{Name: "test", Addr: "localhost:8080"}
    
    mock.Register(ctx, info)
    mock.GetService(ctx, "test")
    mock.GetServiceList(ctx, "test")
    mock.Deregister(ctx, info)
    mock.Close()

    // 验证调用记录
    assert.Len(t, mock.RegisterCalls, 1)
    assert.Len(t, mock.GetServiceCalls, 1)
    assert.Len(t, mock.GetServiceListCalls, 1)
    assert.Len(t, mock.DeregisterCalls, 1)
    assert.Equal(t, 1, mock.CloseCalls)
}
```

### 3. 表驱动测试

```go
func TestRegistry_MultipleScenarios(t *testing.T) {
    tests := []struct {
        name        string
        setupMock   func(*registry.MockRegistry)
        serviceName string
        wantAddr    string
        wantErr     bool
    }{
        {
            name: "service exists",
            setupMock: func(m *registry.MockRegistry) {
                m.Register(context.Background(), &registry.ServiceInfo{
                    Name: "my-service",
                    Addr: "localhost:8080",
                })
            },
            serviceName: "my-service",
            wantAddr:    "localhost:8080",
            wantErr:     false,
        },
        {
            name: "service not found",
            setupMock: func(m *registry.MockRegistry) {
                // 不注册任何服务
            },
            serviceName: "non-existent",
            wantAddr:    "",
            wantErr:     true,
        },
        {
            name: "multiple services",
            setupMock: func(m *registry.MockRegistry) {
                m.Register(context.Background(), &registry.ServiceInfo{
                    Name: "service-1",
                    Addr: "localhost:8080",
                })
                m.Register(context.Background(), &registry.ServiceInfo{
                    Name: "service-2",
                    Addr: "localhost:8081",
                })
            },
            serviceName: "service-2",
            wantAddr:    "localhost:8081",
            wantErr:     false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            mock := registry.NewMockRegistry()
            tt.setupMock(mock)

            addr, err := mock.GetService(context.Background(), tt.serviceName)
            
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
                assert.Equal(t, tt.wantAddr, addr)
            }
        })
    }
}
```

## 集成测试

### 1. 使用真实 etcd 的集成测试

```go
// internal/pkg/registry/etcd_integration_test.go
// +build integration

package registry_test

import (
    "context"
    "testing"
    "time"

    "github.com/serendipityConfusion/notification-platform/internal/pkg/registry"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    clientv3 "go.etcd.io/etcd/client/v3"
)

func setupEtcd(t *testing.T) (*clientv3.Client, func()) {
    client, err := clientv3.New(clientv3.Config{
        Endpoints:   []string{"localhost:2379"},
        DialTimeout: 5 * time.Second,
    })
    require.NoError(t, err)

    cleanup := func() {
        // 清理测试数据
        client.Delete(context.Background(), "/services/", clientv3.WithPrefix())
        client.Close()
    }

    return client, cleanup
}

func TestEtcdRegistry_Integration(t *testing.T) {
    client, cleanup := setupEtcd(t)
    defer cleanup()

    reg := registry.NewEtcdRegistry(client)
    defer reg.Close()

    // 注册服务
    info := &registry.ServiceInfo{
        Name:      "test-service",
        Addr:      "localhost:8080",
        TTL:       10 * time.Second,
        Namespace: "/services",
    }

    ctx := context.Background()
    err := reg.Register(ctx, info)
    require.NoError(t, err)

    // 验证服务已注册到 etcd
    resp, err := client.Get(ctx, "/services/test-service")
    require.NoError(t, err)
    require.Len(t, resp.Kvs, 1)
    assert.Equal(t, "localhost:8080", string(resp.Kvs[0].Value))

    // 获取服务
    addr, err := reg.GetService(ctx, "test-service")
    require.NoError(t, err)
    assert.Equal(t, "localhost:8080", addr)

    // 注销服务
    err = reg.Deregister(ctx, info)
    require.NoError(t, err)

    // 验证服务已从 etcd 删除
    resp, err = client.Get(ctx, "/services/test-service")
    require.NoError(t, err)
    assert.Len(t, resp.Kvs, 0)
}

func TestEtcdRegistry_Lease(t *testing.T) {
    client, cleanup := setupEtcd(t)
    defer cleanup()

    reg := registry.NewEtcdRegistry(client)

    info := &registry.ServiceInfo{
        Name: "test-service",
        Addr: "localhost:8080",
        TTL:  2 * time.Second, // 2秒TTL
    }

    ctx := context.Background()
    err := reg.Register(ctx, info)
    require.NoError(t, err)

    // 立即关闭注册器（停止心跳）
    reg.Close()

    // 等待 TTL 过期
    time.Sleep(3 * time.Second)

    // 验证服务已自动删除
    resp, err := client.Get(ctx, "/services/test-service")
    require.NoError(t, err)
    assert.Len(t, resp.Kvs, 0, "Service should be removed after TTL expires")
}
```

### 2. 运行集成测试

```bash
# 启动 etcd
docker run -d --name etcd-test -p 2379:2379 \
  quay.io/coreos/etcd:latest \
  etcd --listen-client-urls http://0.0.0.0:2379 \
  --advertise-client-urls http://localhost:2379

# 运行集成测试
go test -tags=integration ./internal/pkg/registry/...

# 停止 etcd
docker stop etcd-test && docker rm etcd-test
```

## 自定义实现

### 1. 实现基于 Consul 的 Registry

```go
// internal/pkg/registry/consul.go
package registry

import (
    "context"
    "fmt"
    "strconv"
    "strings"

    "github.com/hashicorp/consul/api"
)

type ConsulRegistry struct {
    client *api.Client
}

func NewConsulRegistry(client *api.Client) *ConsulRegistry {
    return &ConsulRegistry{client: client}
}

func (r *ConsulRegistry) Register(ctx context.Context, info *ServiceInfo) error {
    host, portStr, err := parseAddr(info.Addr)
    if err != nil {
        return err
    }
    
    port, _ := strconv.Atoi(portStr)

    registration := &api.AgentServiceRegistration{
        ID:      info.Name,
        Name:    info.Name,
        Address: host,
        Port:    port,
        Tags:    r.buildTags(info.Metadata),
        Check: &api.AgentServiceCheck{
            TTL:                            info.TTL.String(),
            DeregisterCriticalServiceAfter: "30s",
        },
    }

    return r.client.Agent().ServiceRegister(registration)
}

func (r *ConsulRegistry) Deregister(ctx context.Context, info *ServiceInfo) error {
    return r.client.Agent().ServiceDeregister(info.Name)
}

func (r *ConsulRegistry) Close() error {
    return nil
}

func (r *ConsulRegistry) GetService(ctx context.Context, name string) (string, error) {
    services, _, err := r.client.Health().Service(name, "", true, nil)
    if err != nil {
        return "", err
    }
    if len(services) == 0 {
        return "", fmt.Errorf("service %s not found", name)
    }
    
    svc := services[0].Service
    return fmt.Sprintf("%s:%d", svc.Address, svc.Port), nil
}

func (r *ConsulRegistry) buildTags(metadata map[string]string) []string {
    tags := make([]string, 0, len(metadata))
    for k, v := range metadata {
        tags = append(tags, fmt.Sprintf("%s=%s", k, v))
    }
    return tags
}

func parseAddr(addr string) (host, port string, err error) {
    parts := strings.Split(addr, ":")
    if len(parts) != 2 {
        return "", "", fmt.Errorf("invalid address: %s", addr)
    }
    return parts[0], parts[1], nil
}

var _ Registry = (*ConsulRegistry)(nil)
var _ DiscoveryRegistry = (*ConsulRegistry)(nil)
```

### 2. 配置 Wire 使用 Consul

```go
// internal/ioc/consul.go
package ioc

import (
    "github.com/hashicorp/consul/api"
    "github.com/serendipityConfusion/notification-platform/internal/pkg/registry"
)

func InitConsulClient() (*api.Client, error) {
    config := api.DefaultConfig()
    config.Address = "localhost:8500"
    return api.NewClient(config)
}

func InitConsulRegistry(client *api.Client) *registry.ConsulRegistry {
    return registry.NewConsulRegistry(client)
}
```

```go
// cmd/platform/ioc/wire.go
var RegistrySet = wire.NewSet(
    ioc.InitConsulClient,
    ioc.InitConsulRegistry,
    ioc.InitConfigLoader,
    ioc.InitServiceInfo,
    wire.Bind(new(registry.Registry), new(*registry.ConsulRegistry)), // 切换到 Consul
    wire.Bind(new(config.ConfigLoader), new(*config.ViperConfigLoader)),
)
```

## 高级用法

### 1. 服务发现与监听

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/serendipityConfusion/notification-platform/internal/pkg/registry"
    clientv3 "go.etcd.io/etcd/client/v3"
)

func main() {
    client, _ := clientv3.New(clientv3.Config{
        Endpoints:   []string{"localhost:2379"},
        DialTimeout: 5 * time.Second,
    })
    defer client.Close()

    reg := registry.NewEtcdRegistry(client)

    // 监听服务变化
    ctx := context.Background()
    eventCh, err := reg.Watch(ctx, "notification-server")
    if err != nil {
        log.Fatal(err)
    }

    for event := range eventCh {
        switch event.Type {
        case registry.EventTypeAdd:
            log.Printf("Service added: %s -> %s", 
                event.Service.Name, event.Service.Addr)
            // 可以在这里建立新连接
        case registry.EventTypeDelete:
            log.Printf("Service deleted: %s", event.Service.Name)
            // 可以在这里关闭连接
        case registry.EventTypeUpdate:
            log.Printf("Service updated: %s -> %s", 
                event.Service.Name, event.Service.Addr)
        }
    }
}
```

### 2. 带元数据的服务注册

```go
func registerWithMetadata() {
    reg := registry.NewEtcdRegistry(client)

    info := &registry.ServiceInfo{
        Name: "api-gateway",
        Addr: "localhost:8080",
        TTL:  10 * time.Second,
        Metadata: map[string]string{
            "version":     "1.0.0",
            "region":      "us-west-2",
            "zone":        "zone-a",
            "weight":      "100",
            "tags":        "prod,public",
            "proto":       "grpc",
            "cpu_cores":   "4",
            "memory_mb":   "8192",
        },
    }

    if err := reg.Register(context.Background(), info); err != nil {
        log.Fatal(err)
    }
}
```

### 3. 环境配置切换

```go
// config/dev/config.yaml
etcd:
  endpoints: ["localhost:2379"]

// config/prod/config.yaml  
etcd:
  endpoints: ["etcd-1:2379", "etcd-2:2379", "etcd-3:2379"]
  username: "admin"
  password: "${ETCD_PASSWORD}"

// main.go
func main() {
    env := os.Getenv("ENV")
    if env == "" {
        env = "dev"
    }

    configPath := fmt.Sprintf("./config/%s", env)
    if err := config.InitViperConfig(configPath); err != nil {
        log.Fatal(err)
    }

    app := ioc.InitGrpcServer()
    app.Run()
}
```

### 4. 优雅重启

```go
func (a *App) Reload() error {
    log.Println("Reloading application...")

    // 1. 从注册中心注销旧服务
    if err := a.Registry.Deregister(context.Background(), a.ServiceInfo); err != nil {
        return err
    }

    // 2. 重新加载配置
    grpcConf := &config.GrpcConfig{}
    if err := a.ConfigLoader.Load("notification-server", grpcConf); err != nil {
        return err
    }

    // 3. 更新服务信息
    a.ServiceInfo.Addr = grpcConf.Addr

    // 4. 重新注册
    if err := a.Registry.Register(context.Background(), a.ServiceInfo); err != nil {
        return err
    }

    log.Println("Application reloaded successfully")
    return nil
}
```

## 最佳实践

### 1. 使用接口而非具体类型

```go
// ✅ 好的做法
type Service struct {
    registry registry.Registry
    config   config.ConfigLoader
}

// ❌ 不好的做法
type Service struct {
    registry *registry.EtcdRegistry
    config   *config.ViperConfigLoader
}
```

### 2. 单元测试使用 Mock

```go
// ✅ 好的做法
func TestMyService(t *testing.T) {
    mockReg := registry.NewMockRegistry()
    svc := NewService(mockReg)
    // 测试逻辑...
}

// ❌ 不好的做法
func TestMyService(t *testing.T) {
    etcdClient := ... // 需要真实 etcd
    reg := registry.NewEtcdRegistry(etcdClient)
    svc := NewService(reg)
    // 测试逻辑...
}
```

### 3. 使用 Wire 管理依赖

```go
// ✅ 好的做法：在 Wire 中声明依赖
var MyServiceSet = wire.NewSet(
    NewMyService,
    wire.Bind(new(ServiceInterface), new(*MyService)),
)

// ❌ 不好的做法：手动创建依赖
func main() {
    client := InitEtcdClient()
    reg := registry.NewEtcdRegistry(client)
    loader := config.NewViperConfigLoader()
    svc := NewMyService(reg, loader)
    // ...
}
```

### 4. 配置验证

```go
func (a *App) validateConfig() error {
    if a.ServiceInfo == nil {
        return fmt.Errorf("service info is required")
    }
    if a.ServiceInfo.Name == "" {
        return fmt.Errorf("service name is required")
    }
    if a.ServiceInfo.Addr == "" {
        return fmt.Errorf("service address is required")
    }
    return nil
}

func (a *App) Run() error {
    if err := a.validateConfig(); err != nil {
        return fmt.Errorf("invalid configuration: %w", err)
    }
    // ...
}
```

## 总结

本文档提供了优化后架构的完整使用示例，包括：

- ✅ 基本使用方法
- ✅ 单元测试和集成测试
- ✅ 自定义实现（Consul）
- ✅ 高级用法（监听、元数据、配置切换）
- ✅ 最佳实践

通过这些示例，你可以：
1. 快速上手使用新架构
2. 编写高质量的测试代码
3. 根据需求扩展功能
4. 遵循最佳实践

如有问题，请参考：
- [架构优化说明](./architecture-optimization.md)
- [服务注册文档](./service-registration.md)
- [快速开始指南](./quick-start.md)