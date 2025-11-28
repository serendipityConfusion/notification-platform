# 架构优化说明文档

## 优化概述

本次优化基于 **依赖注入（Dependency Injection）** 和 **面向接口编程** 的设计原则，使用 Google Wire 框架对服务注册和配置加载进行了抽象化改造。

### 优化目标

1. **解耦具体实现**：通过接口抽象，降低模块间的耦合度
2. **提高可测试性**：便于编写单元测试和集成测试
3. **增强可扩展性**：轻松替换不同的注册中心和配置源
4. **符合 SOLID 原则**：尤其是依赖倒置原则（DIP）和开闭原则（OCP）
5. **统一依赖管理**：使用 Wire 自动化依赖注入

## 优化前的架构问题

### 1. 硬编码依赖

**问题代码示例：**
```go
type App struct {
    GrpcServer *grpc.Server
    EtcdClient *clientv3.Client  // 直接依赖 etcd 客户端
}

func (a *App) Run() error {
    // 直接使用 viper 全局实例
    conf := &config.GrpcConfig{}
    viper.UnmarshalKey("notification-server", conf)
    
    // 直接操作 etcd API
    leaseResp, _ := a.EtcdClient.Grant(ctx, 10)
    a.EtcdClient.Put(ctx, key, value, clientv3.WithLease(leaseResp.ID))
}
```

**存在的问题：**
- App 直接依赖 etcd 的具体实现
- 无法替换为其他注册中心（Consul, Nacos 等）
- 难以进行单元测试（需要真实的 etcd）
- 配置加载逻辑散落在各处

### 2. 职责不清晰

- App 需要了解服务注册的细节（lease、心跳等）
- 配置加载逻辑和业务逻辑混杂
- 难以复用服务注册逻辑

### 3. 测试困难

```go
// 测试时必须启动真实的 etcd
func TestApp_Run(t *testing.T) {
    // 需要连接真实的 etcd...
    client, _ := clientv3.New(...)
    app := &App{EtcdClient: client}
    // ...
}
```

## 优化后的架构

### 1. 核心抽象层

#### Registry 接口（服务注册抽象）

```go
// 服务信息结构
type ServiceInfo struct {
    Name      string
    Addr      string
    Metadata  map[string]string
    TTL       time.Duration
    Namespace string
}

// 服务注册接口
type Registry interface {
    Register(ctx context.Context, info *ServiceInfo) error
    Deregister(ctx context.Context, info *ServiceInfo) error
    Close() error
}

// 扩展接口：支持服务发现
type DiscoveryRegistry interface {
    Registry
    GetService(ctx context.Context, name string) (string, error)
    GetServiceList(ctx context.Context, name string) ([]string, error)
    Watch(ctx context.Context, name string) (<-chan Event, error)
}
```

#### ConfigLoader 接口（配置加载抽象）

```go
type ConfigLoader interface {
    Load(key string, target interface{}) error
    GetString(key string) string
    GetInt(key string) int
    GetBool(key string) bool
    GetDuration(key string) time.Duration
}
```

### 2. 具体实现层

#### EtcdRegistry（etcd 实现）

```go
type EtcdRegistry struct {
    client      *clientv3.Client
    leaseID     clientv3.LeaseID
    keepAliveCh <-chan *clientv3.LeaseKeepAliveResponse
    registered  map[string]*ServiceInfo
}

func NewEtcdRegistry(client *clientv3.Client) *EtcdRegistry {
    // 实现细节...
}

func (r *EtcdRegistry) Register(ctx context.Context, info *ServiceInfo) error {
    // 封装 etcd 注册逻辑
    // - 创建 lease
    // - 注册服务
    // - 启动心跳
}
```

#### ViperConfigLoader（Viper 实现）

```go
type ViperConfigLoader struct {
    v *viper.Viper
}

func NewViperConfigLoader() *ViperConfigLoader {
    return &ViperConfigLoader{v: viper.GetViper()}
}

func (l *ViperConfigLoader) Load(key string, target interface{}) error {
    // 封装 Viper 加载逻辑
}
```

### 3. 应用层（使用抽象）

```go
type App struct {
    GrpcServer   *grpc.Server
    Registry     registry.Registry        // 依赖抽象接口
    ConfigLoader config.ConfigLoader      // 依赖抽象接口
    ServiceInfo  *registry.ServiceInfo
}

func (a *App) Run() error {
    // 使用抽象接口，不关心具体实现
    grpcConf := &config.GrpcConfig{}
    a.ConfigLoader.Load("notification-server", grpcConf)
    
    a.ServiceInfo = &registry.ServiceInfo{
        Name: grpcConf.Name,
        Addr: grpcConf.Addr,
    }
    
    a.Registry.Register(ctx, a.ServiceInfo)
    // ...
}
```

### 4. Wire 依赖注入配置

```go
// wire.go
var RegistrySet = wire.NewSet(
    ioc.InitRegistry,
    ioc.InitConfigLoader,
    ioc.InitServiceInfo,
    wire.Bind(new(registry.Registry), new(*registry.EtcdRegistry)),
    wire.Bind(new(config.ConfigLoader), new(*config.ViperConfigLoader)),
)

func InitGrpcServer() *ioc.App {
    wire.Build(
        BaseSet,
        RegistrySet,
        notificationSvcSet,
        grpcapi.NewServer,
        ioc.InitGrpc,
        wire.Struct(new(ioc.App), "*"),
    )
    return &ioc.App{}
}
```

## 核心设计原则

### 1. 依赖倒置原则（DIP）

**高层模块不应该依赖低层模块，两者都应该依赖抽象**

```
优化前：
App (高层) → EtcdClient (低层)

优化后：
App (高层) → Registry (抽象) ← EtcdRegistry (低层)
```

### 2. 开闭原则（OCP）

**对扩展开放，对修改关闭**

添加新的注册中心实现（如 Consul）时：
```go
// 1. 实现 Registry 接口
type ConsulRegistry struct { ... }
func (r *ConsulRegistry) Register(...) error { ... }

// 2. 在 Wire 中切换绑定
wire.Bind(new(registry.Registry), new(*registry.ConsulRegistry))

// 3. App 代码无需任何修改！
```

### 3. 接口隔离原则（ISP）

```go
// 基础接口：只关心注册
type Registry interface {
    Register(...) error
    Deregister(...) error
}

// 扩展接口：需要发现功能时才使用
type DiscoveryRegistry interface {
    Registry
    GetService(...) (string, error)
    Watch(...) (<-chan Event, error)
}
```

### 4. 单一职责原则（SRP）

- `Registry`: 只负责服务注册/注销
- `ConfigLoader`: 只负责配置加载
- `App`: 只负责应用生命周期管理

## 主要改进点

### 1. 可测试性显著提升

#### Mock 测试示例

```go
// mock_registry.go
type MockRegistry struct {
    RegisterFunc   func(ctx context.Context, info *ServiceInfo) error
    DeregisterFunc func(ctx context.Context, info *ServiceInfo) error
}

func (m *MockRegistry) Register(ctx context.Context, info *ServiceInfo) error {
    if m.RegisterFunc != nil {
        return m.RegisterFunc(ctx, info)
    }
    return nil
}

// app_test.go
func TestApp_Run(t *testing.T) {
    mockRegistry := &MockRegistry{
        RegisterFunc: func(ctx context.Context, info *ServiceInfo) error {
            assert.Equal(t, "notification-server", info.Name)
            return nil
        },
    }
    
    mockConfig := &MockConfigLoader{ ... }
    
    app := &App{
        Registry:     mockRegistry,
        ConfigLoader: mockConfig,
        // ...
    }
    
    // 无需真实的 etcd 即可测试！
    err := app.Run()
    assert.NoError(t, err)
}
```

### 2. 扩展性增强

#### 切换到 Consul

```go
// 1. 实现 Registry 接口
type ConsulRegistry struct {
    client *api.Client
}

func (r *ConsulRegistry) Register(ctx context.Context, info *ServiceInfo) error {
    registration := &api.AgentServiceRegistration{
        ID:      info.Name,
        Name:    info.Name,
        Address: info.Addr,
        Check: &api.AgentServiceCheck{
            TTL: info.TTL.String(),
        },
    }
    return r.client.Agent().ServiceRegister(registration)
}

// 2. Wire 配置切换
func InitRegistry(consulClient *api.Client) *ConsulRegistry {
    return NewConsulRegistry(consulClient)
}

// 绑定切换
wire.Bind(new(registry.Registry), new(*registry.ConsulRegistry))
```

#### 支持多配置源

```go
// Nacos 配置加载器
type NacosConfigLoader struct {
    client config_client.IConfigClient
}

func (l *NacosConfigLoader) Load(key string, target interface{}) error {
    content, err := l.client.GetConfig(vo.ConfigParam{
        DataId: key,
        Group:  "DEFAULT_GROUP",
    })
    return yaml.Unmarshal([]byte(content), target)
}

// Wire 绑定
wire.Bind(new(config.ConfigLoader), new(*config.NacosConfigLoader))
```

### 3. 代码复用性

#### 在其他服务中复用

```go
// 在任何需要服务注册的地方
func StartMyService() {
    registry := registry.NewEtcdRegistry(etcdClient)
    
    info := &registry.ServiceInfo{
        Name: "my-service",
        Addr: "localhost:9090",
        TTL:  10 * time.Second,
    }
    
    registry.Register(context.Background(), info)
    defer registry.Deregister(context.Background(), info)
    
    // 启动服务...
}
```

### 4. 配置管理统一化

```go
// 所有配置加载都通过 ConfigLoader
type ServiceConfig struct {
    Grpc     *config.GrpcConfig
    Database *config.DatabaseConfig
    Redis    *config.RedisConfig
}

func LoadAllConfig(loader config.ConfigLoader) (*ServiceConfig, error) {
    cfg := &ServiceConfig{}
    
    if err := loader.Load("grpc", &cfg.Grpc); err != nil {
        return nil, err
    }
    if err := loader.Load("database", &cfg.Database); err != nil {
        return nil, err
    }
    if err := loader.Load("redis", &cfg.Redis); err != nil {
        return nil, err
    }
    
    return cfg, nil
}
```

## Wire 依赖注入优势

### 1. 编译时检查

```bash
# Wire 在编译时检查依赖是否满足
$ wire
wire: /path/to/wire.go:10:1: inject InitApp: no provider found for SomeInterface
```

**对比运行时注入框架（如 dig）：** 运行时才发现依赖缺失

### 2. 零运行时开销

Wire 生成的是普通 Go 代码：
```go
// wire_gen.go (自动生成)
func InitGrpcServer() *ioc.App {
    db := ioc.InitDB()
    client := ioc.InitRedis()
    registry := ioc.InitRegistry(etcdClient)
    loader := ioc.InitConfigLoader()
    app := &ioc.App{
        Registry:     registry,
        ConfigLoader: loader,
        // ...
    }
    return app
}
```

无反射、无性能损耗！

### 3. 类型安全

```go
// 编译时即可发现类型错误
wire.Bind(new(registry.Registry), new(*registry.EtcdRegistry))
//        ^期望类型                ^实际类型
// 如果 EtcdRegistry 没实现 Registry，编译失败
```

### 4. 清晰的依赖关系

```go
var RegistrySet = wire.NewSet(
    ioc.InitRegistry,          // 提供 *EtcdRegistry
    ioc.InitConfigLoader,      // 提供 *ViperConfigLoader
    ioc.InitServiceInfo,       // 提供 *ServiceInfo
    wire.Bind(...),            // 接口绑定
)

// 依赖关系一目了然
```

## 架构层次

```
┌─────────────────────────────────────────────────┐
│              Application Layer                   │
│  ┌─────────────────────────────────────────┐   │
│  │          App (Business Logic)            │   │
│  └─────────────────────────────────────────┘   │
└────────────────┬──────────────┬────────────────┘
                 │              │
         依赖抽象接口        依赖抽象接口
                 │              │
                 ▼              ▼
┌─────────────────────────────────────────────────┐
│              Abstraction Layer                   │
│  ┌──────────────┐          ┌──────────────┐    │
│  │   Registry   │          │ ConfigLoader │    │
│  │  (Interface) │          │  (Interface) │    │
│  └──────────────┘          └──────────────┘    │
└────────────────┬──────────────┬────────────────┘
                 │              │
           实现接口           实现接口
                 │              │
                 ▼              ▼
┌─────────────────────────────────────────────────┐
│            Implementation Layer                  │
│  ┌──────────────┐          ┌──────────────┐    │
│  │ EtcdRegistry │          │ViperConfig   │    │
│  │ ConsulReg... │          │ NacosConfig  │    │
│  │ NacosReg...  │          │ ...          │    │
│  └──────────────┘          └──────────────┘    │
└─────────────────────────────────────────────────┘
                 │                      │
                 ▼                      ▼
┌─────────────────────────────────────────────────┐
│            Infrastructure Layer                  │
│         (etcd, consul, viper, etc.)             │
└─────────────────────────────────────────────────┘
```

## 对比总结

| 维度 | 优化前 | 优化后 |
|------|--------|--------|
| **依赖关系** | App → EtcdClient (具体) | App → Registry (抽象) |
| **可测试性** | 需要真实 etcd | 可使用 Mock |
| **扩展性** | 修改 App 代码 | 实现接口即可 |
| **配置加载** | 直接使用 viper 全局变量 | 通过 ConfigLoader 接口 |
| **依赖注入** | 手动 new | Wire 自动生成 |
| **编译检查** | 运行时发现问题 | 编译时发现问题 |
| **代码复用** | 逻辑分散，难复用 | 接口清晰，易复用 |
| **职责分离** | App 关心注册细节 | App 只关心业务逻辑 |

## 最佳实践

### 1. 接口设计原则

- **最小化接口**：一个接口只包含必要的方法
- **职责单一**：一个接口只做一件事
- **易于测试**：方法签名便于 mock

### 2. Provider 命名规范

```go
// 初始化函数统一命名为 Init + 类型名
func InitRegistry(client *clientv3.Client) *registry.EtcdRegistry
func InitConfigLoader() *config.ViperConfigLoader
func InitServiceInfo() *registry.ServiceInfo
```

### 3. Wire Set 组织

```go
// 按功能模块组织 Set
var (
    // 基础设施
    BaseSet = wire.NewSet(InitDB, InitRedis, InitEtcd)
    
    // 服务注册
    RegistrySet = wire.NewSet(InitRegistry, InitConfigLoader)
    
    // 业务服务
    ServiceSet = wire.NewSet(NewUserService, NewOrderService)
)
```

### 4. 接口绑定最佳实践

```go
// 明确的接口绑定
wire.Bind(new(registry.Registry), new(*registry.EtcdRegistry))
//        ^要绑定的接口        ^具体实现类型

// 可以在不同环境使用不同实现
// 开发环境
wire.Bind(new(registry.Registry), new(*registry.MockRegistry))
// 生产环境
wire.Bind(new(registry.Registry), new(*registry.EtcdRegistry))
```

## 扩展示例

### 添加新的注册中心 - Consul

#### 1. 实现接口

```go
// internal/pkg/registry/consul.go
package registry

import (
    "context"
    "fmt"
    "github.com/hashicorp/consul/api"
)

type ConsulRegistry struct {
    client *api.Client
}

func NewConsulRegistry(client *api.Client) *ConsulRegistry {
    return &ConsulRegistry{client: client}
}

func (r *ConsulRegistry) Register(ctx context.Context, info *ServiceInfo) error {
    registration := &api.AgentServiceRegistration{
        ID:      info.Name,
        Name:    info.Name,
        Address: parseAddr(info.Addr),
        Port:    parsePort(info.Addr),
        Check: &api.AgentServiceCheck{
            TTL: info.TTL.String(),
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

// 确保实现了接口
var _ Registry = (*ConsulRegistry)(nil)
```

#### 2. 添加 Provider

```go
// internal/ioc/registry.go
func InitConsulRegistry() *registry.ConsulRegistry {
    config := api.DefaultConfig()
    config.Address = "localhost:8500"
    
    client, err := api.NewClient(config)
    if err != nil {
        panic(err)
    }
    
    return registry.NewConsulRegistry(client)
}
```

#### 3. 切换绑定

```go
// cmd/platform/ioc/wire.go
var RegistrySet = wire.NewSet(
    ioc.InitConsulRegistry,  // 改：使用 Consul
    ioc.InitConfigLoader,
    ioc.InitServiceInfo,
    wire.Bind(new(registry.Registry), new(*registry.ConsulRegistry)),  // 改：绑定 Consul
    wire.Bind(new(config.ConfigLoader), new(*config.ViperConfigLoader)),
)
```

#### 4. 重新生成

```bash
cd cmd/platform/ioc && wire
```

**完成！** App 代码无需任何修改即可切换到 Consul！

## 总结

通过本次架构优化：

✅ **实现了真正的依赖注入**：使用 Wire 自动化管理依赖

✅ **面向接口编程**：高层模块依赖抽象而非具体实现

✅ **提升了可测试性**：可以轻松使用 Mock 进行单元测试

✅ **增强了扩展性**：添加新实现无需修改现有代码

✅ **职责更加清晰**：每个模块只关注自己的职责

✅ **代码更易维护**：依赖关系清晰，易于理解和修改

✅ **符合 SOLID 原则**：尤其是 DIP 和 OCP

这是一次典型的 **从具体到抽象** 的重构过程，是现代 Go 项目架构的最佳实践。