# 服务注册与发现

## 概述

本项目实现了基于 etcd 的服务注册与发现功能。当应用启动时，会自动将 gRPC 服务地址注册到 etcd，并在应用关闭时自动注销。

## 功能特性

- ✅ 自动服务注册：启动时自动将服务地址注册到 etcd
- ✅ 心跳保持：使用 etcd lease 机制保持服务在线状态
- ✅ 优雅关闭：应用关闭时自动从 etcd 注销服务
- ✅ 故障恢复：如果应用异常退出，lease 过期后 etcd 会自动删除服务记录

## 配置说明

### 1. gRPC 服务配置

在 `config/platform/config.yaml` 中配置 gRPC 服务信息：

```yaml
notification-server:
  addr: "0.0.0.0:8080"  # gRPC 服务监听地址
  name: "notification-server"  # 服务名称，用于在 etcd 中标识服务
```

### 2. etcd 配置

在同一配置文件中配置 etcd 连接信息：

```yaml
etcd:
  endpoints: ["localhost:2379"]  # etcd 服务地址列表
  dial-timeout: 5s  # 连接超时时间
  username: ""  # 可选：etcd 用户名
  password: ""  # 可选：etcd 密码
```

## 工作原理

### 服务注册流程

1. **读取配置**：从配置文件中读取 gRPC 服务地址和服务名称
2. **创建租约**：向 etcd 申请一个 10 秒 TTL 的租约
3. **注册服务**：将服务信息写入 etcd，格式为 `/services/{service_name}` -> `{service_addr}`
4. **保持心跳**：后台持续向 etcd 发送 keep-alive 请求续约
5. **启动服务**：启动 gRPC 服务监听请求

### 服务注销流程

1. **监听信号**：监听 `SIGINT` 和 `SIGTERM` 信号
2. **删除注册**：从 etcd 中删除服务注册信息
3. **撤销租约**：撤销 etcd 租约
4. **优雅停止**：调用 gRPC 的 `GracefulStop()` 方法停止服务

### etcd 中的数据结构

```
/services/notification-server -> "0.0.0.0:8080"
```

## 使用方法

### 启动应用

```bash
cd cmd/platform
go run main.go
```

应用启动后，你会看到类似以下日志：

```
2024/01/01 10:00:00 Using config file: config/platform/config.yaml
2024/01/01 10:00:00 Service registered to etcd: /services/notification-server -> 0.0.0.0:8080
2024/01/01 10:00:00 gRPC server listening on 0.0.0.0:8080
```

### 停止应用

按 `Ctrl+C` 或发送 `SIGTERM` 信号：

```bash
kill -TERM <pid>
```

应用会优雅关闭并输出：

```
2024/01/01 10:00:00 Shutting down server...
2024/01/01 10:00:00 Service deregistered from etcd: /services/notification-server
2024/01/01 10:00:00 Server stopped gracefully
```

### 查看 etcd 中的服务

使用 etcdctl 查看注册的服务：

```bash
# 查看所有服务
etcdctl get /services/ --prefix

# 查看特定服务
etcdctl get /services/notification-server
```

## 代码结构

### App 结构体

```go
type App struct {
    GrpcServer *grpc.Server      // gRPC 服务器实例
    EtcdClient *clientv3.Client  // etcd 客户端
}
```

### Run 方法

`App.Run()` 方法负责：
- 从配置中读取 gRPC 地址
- 将服务注册到 etcd
- 启动 gRPC 服务器
- 监听退出信号
- 优雅关闭服务

## 最佳实践

### 1. 租约时间设置

当前租约 TTL 设置为 10 秒，这意味着：
- 如果应用正常运行，每隔几秒会自动续约
- 如果应用异常退出，最多 10 秒后服务记录会被自动删除
- 可以根据实际需求调整 TTL 时间

### 2. 服务发现

其他服务可以通过以下方式发现服务：

```go
// 获取服务地址
resp, err := etcdClient.Get(ctx, "/services/notification-server")
if err != nil {
    // 处理错误
}
if len(resp.Kvs) > 0 {
    serviceAddr := string(resp.Kvs[0].Value)
    // 使用 serviceAddr 连接服务
}

// 监听服务变化
watchChan := etcdClient.Watch(ctx, "/services/", clientv3.WithPrefix())
for wresp := range watchChan {
    for _, ev := range wresp.Events {
        // 处理服务上线/下线事件
    }
}
```

### 3. 多实例部署

如果需要部署多个实例，建议修改服务 key 的格式：

```go
serviceKey := fmt.Sprintf("/services/%s/%s", conf.Name, instanceID)
```

其中 `instanceID` 可以是：
- 主机名
- IP 地址
- UUID
- Pod 名称（Kubernetes 环境）

## 故障排查

### 问题：服务注册失败

**可能原因**：
- etcd 服务未启动
- etcd 配置错误
- 网络连接问题

**解决方法**：
1. 检查 etcd 是否运行：`etcdctl endpoint health`
2. 检查配置文件中的 endpoints 是否正确
3. 检查网络连接和防火墙设置

### 问题：服务异常退出后仍在 etcd 中

**可能原因**：
- 租约 TTL 未过期

**解决方法**：
- 等待租约 TTL 时间后自动删除（默认 10 秒）
- 或手动删除：`etcdctl del /services/notification-server`

### 问题：心跳续约失败

**可能原因**：
- 网络中断
- etcd 服务异常

**解决方法**：
- 检查应用日志中的 "Lease keep-alive channel closed" 消息
- 实现重连机制（可选）

## 扩展功能

### 添加健康检查

可以在注册服务时添加健康检查信息：

```go
healthInfo := map[string]string{
    "addr": conf.Addr,
    "status": "healthy",
    "version": "1.0.0",
}
jsonData, _ := json.Marshal(healthInfo)
_, err = a.EtcdClient.Put(ctx, serviceKey, string(jsonData), clientv3.WithLease(leaseResp.ID))
```

### 添加负载均衡

结合 gRPC 的 resolver 机制，可以实现基于 etcd 的客户端负载均衡：

```go
// 实现自定义 resolver，从 etcd 获取服务列表
// 参考：https://github.com/etcd-io/etcd/tree/main/client/v3/naming
```

## 参考资料

- [etcd 官方文档](https://etcd.io/docs/)
- [gRPC Go 文档](https://grpc.io/docs/languages/go/)
- [etcd client v3](https://pkg.go.dev/go.etcd.io/etcd/client/v3)