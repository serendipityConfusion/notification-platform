# 快速开始指南

## 前置要求

在开始之前，请确保已安装以下组件：

- Go 1.19+
- etcd 3.5+（或使用 Docker 运行）
- etcdctl（用于验证）

## 5 分钟快速启动

### 1. 启动 etcd

使用 Docker 快速启动 etcd：

```bash
docker run -d --name etcd \
  -p 2379:2379 \
  -p 2380:2380 \
  quay.io/coreos/etcd:latest \
  etcd \
  --listen-client-urls http://0.0.0.0:2379 \
  --advertise-client-urls http://localhost:2379
```

验证 etcd 是否运行：

```bash
etcdctl endpoint health
# 输出: localhost:2379 is healthy: successfully committed proposal: took = 1.234ms
```

### 2. 配置应用

确保配置文件 `config/platform/config.yaml` 包含以下内容：

```yaml
mysql:
  dsn: "root:root@tcp(localhost:13316)/notification?charset=utf8mb4&collation=utf8mb4_general_ci&parseTime=True&loc=Local&timeout=1s&readTimeout=3s&writeTimeout=3s&multiStatements=true&interpolateParams=true"

redis:
  addr: "localhost:6379"
  password: ""

notification-server:
  addr: "0.0.0.0:8080"
  name: "notification-server"

etcd:
  endpoints: ["localhost:2379"]
  dial-timeout: 5s
```

### 3. 启动应用

```bash
# 进入项目目录
cd cmd/platform

# 运行应用
go run main.go
```

你应该看到以下输出：

```
2024/01/01 10:00:00 Using config file: ../../config/platform/config.yaml
2024/01/01 10:00:00 Service registered to etcd: /services/notification-server -> 0.0.0.0:8080
2024/01/01 10:00:00 gRPC server listening on 0.0.0.0:8080
```

### 4. 验证服务注册

在另一个终端窗口，查看 etcd 中的服务注册信息：

```bash
# 查看所有注册的服务
etcdctl get /services/ --prefix

# 输出示例:
# /services/notification-server
# 0.0.0.0:8080
```

### 5. 测试优雅关闭

按 `Ctrl+C` 停止应用，你应该看到：

```
^C2024/01/01 10:00:00 Shutting down server...
2024/01/01 10:00:00 Service deregistered from etcd: /services/notification-server
2024/01/01 10:00:00 Server stopped gracefully
```

再次查看 etcd，服务应该已被删除：

```bash
etcdctl get /services/notification-server
# 无输出（服务已注销）
```

## 使用服务发现

### 在客户端中发现服务

创建一个简单的客户端程序：

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/serendipityConfusion/notification-platform/internal/pkg/discovery"
    clientv3 "go.etcd.io/etcd/client/v3"
)

func main() {
    // 1. 创建 etcd 客户端
    client, err := clientv3.New(clientv3.Config{
        Endpoints:   []string{"localhost:2379"},
        DialTimeout: 5 * time.Second,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // 2. 创建服务发现客户端
    sd := discovery.NewServiceDiscovery(client)

    // 3. 获取服务地址
    ctx := context.Background()
    addr, err := sd.GetService(ctx, "notification-server")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Found service at: %s\n", addr)

    // 4. 创建 gRPC 连接
    conn, err := sd.DialService(ctx, "notification-server")
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()

    fmt.Println("Successfully connected to service!")
}
```

## 常见问题

### Q: 服务无法注册到 etcd

**A:** 检查以下几点：
1. etcd 是否正在运行：`etcdctl endpoint health`
2. 配置文件中的 endpoints 是否正确
3. 网络连接是否正常
4. 防火墙是否阻止了 2379 端口

### Q: 应用异常退出后，服务仍在 etcd 中

**A:** 这是正常的。etcd 会在租约过期后（默认 10 秒）自动删除服务记录。

你也可以手动删除：
```bash
etcdctl del /services/notification-server
```

### Q: 如何修改服务监听地址？

**A:** 修改配置文件中的 `notification-server.addr`，然后重启应用。

### Q: 如何部署多个实例？

**A:** 当前版本使用相同的 key，多个实例会覆盖。建议修改代码支持多实例：
```go
serviceKey := fmt.Sprintf("/services/%s/%s", conf.Name, instanceID)
```

其中 `instanceID` 可以是主机名、IP 或 UUID。

## 验证清单

启动成功后，请验证以下内容：

- [ ] 应用启动时输出 "Service registered to etcd"
- [ ] 应用启动时输出 "gRPC server listening on"
- [ ] etcdctl 可以查询到服务：`etcdctl get /services/notification-server`
- [ ] 按 Ctrl+C 后应用输出 "Service deregistered from etcd"
- [ ] 应用停止后，etcd 中的服务记录被删除

## 下一步

恭喜！你已经成功配置并运行了服务注册与发现功能。

接下来你可以：

1. **阅读详细文档**：查看 [service-registration.md](./service-registration.md) 了解更多功能
2. **查看代码示例**：参考 `internal/pkg/discovery/example_test.go` 中的示例
3. **实现服务发现**：在其他服务中使用 discovery 客户端连接此服务
4. **添加监控**：集成 Prometheus 监控服务注册状态
5. **实现负载均衡**：使用多个实例并实现客户端负载均衡

## 故障排查

如果遇到问题，请查看：

1. **应用日志**：检查控制台输出的错误信息
2. **etcd 状态**：`etcdctl endpoint status`
3. **网络连接**：`telnet localhost 2379`
4. **端口占用**：`lsof -i :8080` 或 `netstat -an | grep 8080`

详细的故障排查指南请参考：[service-registration.md](./service-registration.md#故障排查)

## 相关资源

- [完整实现文档](./implementation-summary.md)
- [服务注册详细文档](./service-registration.md)
- [etcd 官方文档](https://etcd.io/docs/latest/)
- [gRPC Go 快速开始](https://grpc.io/docs/languages/go/quickstart/)

## 获取帮助

如果你在使用过程中遇到问题：

1. 查看文档目录中的其他文档
2. 检查代码中的注释
3. 查看示例代码：`internal/pkg/discovery/example_test.go`
4. 提交 Issue 或联系开发团队

祝你使用愉快！🚀