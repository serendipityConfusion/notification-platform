# gRPC API 使用示例

> Notification Platform gRPC API 完整使用指南

**版本**: 1.0 | **更新**: 2024-01 | **状态**: 🟢 可用

---

## 📑 目录

- [概述](#概述)
- [前置准备](#前置准备)
- [通知发送 API](#通知发送-api)
- [查询 API](#查询-api)
- [事务消息 API](#事务消息-api)
- [错误处理](#错误处理)
- [最佳实践](#最佳实践)

---

## 概述

Notification Platform 提供了完整的 gRPC API，支持：

- ✅ **同步/异步发送**：灵活选择发送模式
- ✅ **单条/批量发送**：高效处理不同场景
- ✅ **事务消息**：保证消息可靠性
- ✅ **查询功能**：实时查询发送状态

### API 列表

| API | 功能 | 使用场景 |
|-----|------|----------|
| `SendNotification` | 同步单条发送 | 实时通知，需要立即知道结果 |
| `SendNotificationAsync` | 异步单条发送 | 延迟/定时通知 |
| `BatchSendNotifications` | 同步批量发送 | 批量实时通知 |
| `BatchSendNotificationsAsync` | 异步批量发送 | 批量延迟/定时通知 |
| `TxPrepare` | 准备事务消息 | 分布式事务场景 |
| `TxCommit` | 提交事务消息 | 确认发送 |
| `TxCancel` | 取消事务消息 | 回滚发送 |
| `QueryNotification` | 查询单条通知 | 查询发送状态 |
| `BatchQueryNotifications` | 批量查询通知 | 批量查询状态 |

---

## 前置准备

### 1. 引入 Proto 文件

```bash
# 下载 proto 文件
git clone https://github.com/serendipityConfusion/notification-platform.git
cd notification-platform/api/proto
```

### 2. 生成客户端代码

**Go 客户端：**
```bash
protoc --go_out=. --go-grpc_out=. \
  notification/v1/notification.proto \
  notification/v1/notification_query.proto
```

**Java 客户端：**
```bash
protoc --java_out=. --grpc-java_out=. \
  notification/v1/notification.proto \
  notification/v1/notification_query.proto
```

### 3. 创建客户端连接

```go
package main

import (
    "context"
    "log"
    "time"
    
    notificationpb "github.com/serendipityConfusion/notification-platform/api/gen/v1"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    "google.golang.org/grpc/metadata"
)

func main() {
    // 连接到服务器
    conn, err := grpc.Dial(
        "localhost:8080",
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    if err != nil {
        log.Fatalf("连接失败: %v", err)
    }
    defer conn.Close()
    
    // 创建客户端
    client := notificationpb.NewNotificationServiceClient(conn)
    queryClient := notificationpb.NewNotificationQueryServiceClient(conn)
    
    // 使用客户端...
}
```

### 4. 设置 Metadata（重要）

```go
// 在每个请求中添加 bizID
func withBizID(ctx context.Context, bizID int64) context.Context {
    md := metadata.Pairs("biz-id", fmt.Sprintf("%d", bizID))
    return metadata.NewOutgoingContext(ctx, md)
}

// 使用示例
ctx := withBizID(context.Background(), 12345)
resp, err := client.SendNotification(ctx, req)
```

---

## 通知发送 API

### 1. SendNotification - 同步单条发送

**使用场景**：
- 验证码、短信通知等需要立即发送的场景
- 需要立即知道发送结果
- 对响应时间敏感的场景

**示例代码**：

```go
func sendNotificationSync(client notificationpb.NotificationServiceClient) {
    ctx := withBizID(context.Background(), 12345)
    
    req := &notificationpb.SendNotificationRequest{
        Notification: &notificationpb.Notification{
            Key:        "order-123456",
            Receivers:  []string{"user@example.com"},
            Channel:    notificationpb.Channel_EMAIL,
            TemplateId: "100001",
            TemplateParams: map[string]string{
                "order_id": "123456",
                "amount":   "99.99",
                "product":  "Premium Plan",
            },
            Strategy: &notificationpb.SendStrategy{
                StrategyType: &notificationpb.SendStrategy_Immediate{
                    Immediate: &notificationpb.SendStrategy_ImmediateStrategy{},
                },
            },
        },
    }
    
    resp, err := client.SendNotification(ctx, req)
    if err != nil {
        log.Fatalf("发送失败: %v", err)
    }
    
    fmt.Printf("通知ID: %d\n", resp.NotificationId)
    fmt.Printf("状态: %s\n", resp.Status)
    
    if resp.ErrorCode != notificationpb.ErrorCode_ERROR_CODE_UNSPECIFIED {
        fmt.Printf("错误: %s - %s\n", resp.ErrorCode, resp.ErrorMessage)
    }
}
```

**响应示例**：
```json
{
  "notification_id": 1001,
  "status": "SUCCEEDED",
  "error_code": "ERROR_CODE_UNSPECIFIED",
  "error_message": ""
}
```

---

### 2. SendNotificationAsync - 异步单条发送

**使用场景**：
- 延迟发送（如：10分钟后发送）
- 定时发送（如：明天上午9点发送）
- 截止时间前发送（如：24小时内发送）
- 对响应时间不敏感的场景

**示例 1：延迟发送**

```go
func sendDelayedNotification(client notificationpb.NotificationServiceClient) {
    ctx := withBizID(context.Background(), 12345)
    
    req := &notificationpb.SendNotificationAsyncRequest{
        Notification: &notificationpb.Notification{
            Key:        "reminder-123",
            Receivers:  []string{"13800138000"},
            Channel:    notificationpb.Channel_SMS,
            TemplateId: "100002",
            TemplateParams: map[string]string{
                "event": "会议提醒",
                "time":  "14:00",
            },
            Strategy: &notificationpb.SendStrategy{
                StrategyType: &notificationpb.SendStrategy_Delayed{
                    Delayed: &notificationpb.SendStrategy_DelayedStrategy{
                        DelaySeconds: 600, // 10分钟后发送
                    },
                },
            },
        },
    }
    
    resp, err := client.SendNotificationAsync(ctx, req)
    if err != nil {
        log.Fatalf("创建异步通知失败: %v", err)
    }
    
    fmt.Printf("通知ID: %d (将在10分钟后发送)\n", resp.NotificationId)
}
```

**示例 2：定时发送**

```go
func sendScheduledNotification(client notificationpb.NotificationServiceClient) {
    ctx := withBizID(context.Background(), 12345)
    
    // 明天上午9点
    sendTime := time.Now().AddDate(0, 0, 1)
    sendTime = time.Date(sendTime.Year(), sendTime.Month(), sendTime.Day(), 9, 0, 0, 0, time.Local)
    
    req := &notificationpb.SendNotificationAsyncRequest{
        Notification: &notificationpb.Notification{
            Key:        "daily-report-2024-01-15",
            Receivers:  []string{"admin@example.com"},
            Channel:    notificationpb.Channel_EMAIL,
            TemplateId: "100003",
            TemplateParams: map[string]string{
                "date": "2024-01-15",
            },
            Strategy: &notificationpb.SendStrategy{
                StrategyType: &notificationpb.SendStrategy_Scheduled{
                    Scheduled: &notificationpb.SendStrategy_ScheduledStrategy{
                        SendTime: timestamppb.New(sendTime),
                    },
                },
            },
        },
    }
    
    resp, err := client.SendNotificationAsync(ctx, req)
    if err != nil {
        log.Fatalf("创建定时通知失败: %v", err)
    }
    
    fmt.Printf("通知ID: %d (将在 %s 发送)\n", resp.NotificationId, sendTime)
}
```

**示例 3：截止时间前发送**

```go
func sendWithDeadline(client notificationpb.NotificationServiceClient) {
    ctx := withBizID(context.Background(), 12345)
    
    deadline := time.Now().Add(24 * time.Hour) // 24小时内
    
    req := &notificationpb.SendNotificationAsyncRequest{
        Notification: &notificationpb.Notification{
            Key:        "payment-reminder-456",
            Receivers:  []string{"user@example.com"},
            Channel:    notificationpb.Channel_EMAIL,
            TemplateId: "100004",
            TemplateParams: map[string]string{
                "invoice_id": "INV-456",
                "amount":     "299.99",
            },
            Strategy: &notificationpb.SendStrategy{
                StrategyType: &notificationpb.SendStrategy_Deadline{
                    Deadline: &notificationpb.SendStrategy_DeadlineStrategy{
                        Deadline: timestamppb.New(deadline),
                    },
                },
            },
        },
    }
    
    resp, err := client.SendNotificationAsync(ctx, req)
    if err != nil {
        log.Fatalf("创建通知失败: %v", err)
    }
    
    fmt.Printf("通知ID: %d (将在 %s 前发送)\n", resp.NotificationId, deadline)
}
```

---

### 3. BatchSendNotifications - 同步批量发送

**使用场景**：
- 批量发送相同类型的通知
- 需要立即知道每条通知的发送结果
- 营销活动、系统通知等

**示例代码**：

```go
func batchSendSync(client notificationpb.NotificationServiceClient) {
    ctx := withBizID(context.Background(), 12345)
    
    // 准备批量通知
    notifications := []*notificationpb.Notification{
        {
            Key:        "promo-user-001",
            Receivers:  []string{"user1@example.com"},
            Channel:    notificationpb.Channel_EMAIL,
            TemplateId: "200001",
            TemplateParams: map[string]string{
                "user_name": "张三",
                "discount":  "20%",
            },
            Strategy: &notificationpb.SendStrategy{
                StrategyType: &notificationpb.SendStrategy_Immediate{
                    Immediate: &notificationpb.SendStrategy_ImmediateStrategy{},
                },
            },
        },
        {
            Key:        "promo-user-002",
            Receivers:  []string{"user2@example.com"},
            Channel:    notificationpb.Channel_EMAIL,
            TemplateId: "200001",
            TemplateParams: map[string]string{
                "user_name": "李四",
                "discount":  "20%",
            },
            Strategy: &notificationpb.SendStrategy{
                StrategyType: &notificationpb.SendStrategy_Immediate{
                    Immediate: &notificationpb.SendStrategy_ImmediateStrategy{},
                },
            },
        },
        // ... 更多通知
    }
    
    req := &notificationpb.BatchSendNotificationsRequest{
        Notifications: notifications,
    }
    
    resp, err := client.BatchSendNotifications(ctx, req)
    if err != nil {
        log.Fatalf("批量发送失败: %v", err)
    }
    
    fmt.Printf("总数: %d, 成功: %d\n", resp.TotalCount, resp.SuccessCount)
    
    // 处理每条通知的结果
    for i, result := range resp.Results {
        if result.Status == notificationpb.SendStatus_SUCCEEDED {
            fmt.Printf("[%d] 成功 - ID: %d\n", i, result.NotificationId)
        } else {
            fmt.Printf("[%d] 失败 - %s: %s\n", i, result.ErrorCode, result.ErrorMessage)
        }
    }
}
```

---

### 4. BatchSendNotificationsAsync - 异步批量发送

**使用场景**：
- 大批量通知（如：10万+用户）
- 定时批量发送
- 对响应时间不敏感

**示例代码**：

```go
func batchSendAsync(client notificationpb.NotificationServiceClient) {
    ctx := withBizID(context.Background(), 12345)
    
    // 准备大批量通知
    notifications := make([]*notificationpb.Notification, 0, 1000)
    
    for i := 0; i < 1000; i++ {
        notifications = append(notifications, &notificationpb.Notification{
            Key:        fmt.Sprintf("campaign-2024-user-%d", i),
            Receivers:  []string{fmt.Sprintf("user%d@example.com", i)},
            Channel:    notificationpb.Channel_EMAIL,
            TemplateId: "300001",
            TemplateParams: map[string]string{
                "campaign": "新年促销",
            },
            Strategy: &notificationpb.SendStrategy{
                StrategyType: &notificationpb.SendStrategy_Deadline{
                    Deadline: &notificationpb.SendStrategy_DeadlineStrategy{
                        Deadline: timestamppb.New(time.Now().Add(48 * time.Hour)),
                    },
                },
            },
        })
    }
    
    req := &notificationpb.BatchSendNotificationsAsyncRequest{
        Notifications: notifications,
    }
    
    resp, err := client.BatchSendNotificationsAsync(ctx, req)
    if err != nil {
        log.Fatalf("批量创建异步通知失败: %v", err)
    }
    
    fmt.Printf("成功创建 %d 条异步通知\n", len(resp.NotificationIds))
    fmt.Printf("通知ID范围: %d - %d\n", 
        resp.NotificationIds[0], 
        resp.NotificationIds[len(resp.NotificationIds)-1])
}
```

---

## 查询 API

### 1. QueryNotification - 查询单条通知

**使用场景**：
- 查询通知发送状态
- 跟踪通知发送进度
- 调试和排查问题

**示例代码**：

```go
func queryNotification(client notificationpb.NotificationQueryServiceClient) {
    ctx := withBizID(context.Background(), 12345)
    
    req := &notificationpb.QueryNotificationRequest{
        Key: "order-123456",
    }
    
    resp, err := client.QueryNotification(ctx, req)
    if err != nil {
        log.Fatalf("查询失败: %v", err)
    }
    
    result := resp.Result
    fmt.Printf("通知ID: %d\n", result.NotificationId)
    fmt.Printf("状态: %s\n", result.Status)
    
    switch result.Status {
    case notificationpb.SendStatus_PREPARE:
        fmt.Println("⏳ 事务准备中")
    case notificationpb.SendStatus_PENDING:
        fmt.Println("⏰ 等待发送")
    case notificationpb.SendStatus_SUCCEEDED:
        fmt.Println("✅ 发送成功")
    case notificationpb.SendStatus_FAILED:
        fmt.Printf("❌ 发送失败: %s\n", result.ErrorMessage)
    case notificationpb.SendStatus_CANCELED:
        fmt.Println("🚫 已取消")
    }
}
```

---

### 2. BatchQueryNotifications - 批量查询通知

**使用场景**：
- 批量查询订单通知状态
- 生成发送报表
- 监控发送情况

**示例代码**：

```go
func batchQueryNotifications(client notificationpb.NotificationQueryServiceClient) {
    ctx := withBizID(context.Background(), 12345)
    
    req := &notificationpb.BatchQueryNotificationsRequest{
        Keys: []string{
            "order-123456",
            "order-123457",
            "order-123458",
        },
    }
    
    resp, err := client.BatchQueryNotifications(ctx, req)
    if err != nil {
        log.Fatalf("批量查询失败: %v", err)
    }
    
    fmt.Printf("查询结果（共 %d 条）:\n", len(resp.Results))
    
    for i, result := range resp.Results {
        fmt.Printf("[%d] ID: %d, 状态: %s\n", 
            i+1, 
            result.NotificationId, 
            result.Status)
    }
}
```

---

## 事务消息 API

### 使用场景

事务消息用于保证**分布式事务的最终一致性**：

1. 业务操作和消息发送需要原子性
2. 避免消息丢失或重复发送
3. 支持本地事务失败时回滚消息

### 工作流程

```
业务方                          通知平台
  |                                |
  | 1. TxPrepare(准备消息)          |
  |------------------------------->|
  |                                | (创建 PREPARE 状态的消息)
  |<-------------------------------|
  |                                |
  | 2. 执行本地事务                  |
  |                                |
  | 3a. 事务成功 -> TxCommit        |
  |------------------------------->|
  |                                | (更新为 PENDING，等待发送)
  |<-------------------------------|
  |                                |
  | 3b. 事务失败 -> TxCancel        |
  |------------------------------->|
  |                                | (更新为 CANCELED，不发送)
  |<-------------------------------|
```

### 示例：订单支付场景

```go
func sendPaymentNotificationWithTx(
    client notificationpb.NotificationServiceClient,
    orderID string,
    amount float64,
) error {
    ctx := withBizID(context.Background(), 12345)
    
    // 1. 准备事务消息
    prepareReq := &notificationpb.TxPrepareRequest{
        Notification: &notificationpb.Notification{
            Key:        fmt.Sprintf("payment-%s", orderID),
            Receivers:  []string{"user@example.com"},
            Channel:    notificationpb.Channel_EMAIL,
            TemplateId: "400001",
            TemplateParams: map[string]string{
                "order_id": orderID,
                "amount":   fmt.Sprintf("%.2f", amount),
            },
            Strategy: &notificationpb.SendStrategy{
                StrategyType: &notificationpb.SendStrategy_Immediate{
                    Immediate: &notificationpb.SendStrategy_ImmediateStrategy{},
                },
            },
        },
    }
    
    _, err := client.TxPrepare(ctx, prepareReq)
    if err != nil {
        return fmt.Errorf("准备事务消息失败: %w", err)
    }
    
    // 2. 执行本地事务
    err = executePaymentTransaction(orderID, amount)
    
    // 3. 根据本地事务结果，提交或取消消息
    if err != nil {
        // 本地事务失败，取消消息
        cancelReq := &notificationpb.TxCancelRequest{
            Key: fmt.Sprintf("payment-%s", orderID),
        }
        _, cancelErr := client.TxCancel(ctx, cancelReq)
        if cancelErr != nil {
            log.Printf("取消事务消息失败: %v", cancelErr)
        }
        return fmt.Errorf("支付失败: %w", err)
    }
    
    // 本地事务成功，提交消息
    commitReq := &notificationpb.TxCommitRequest{
        Key: fmt.Sprintf("payment-%s", orderID),
    }
    _, err = client.TxCommit(ctx, commitReq)
    if err != nil {
        return fmt.Errorf("提交事务消息失败: %w", err)
    }
    
    log.Printf("支付成功，通知已发送: %s", orderID)
    return nil
}

func executePaymentTransaction(orderID string, amount float64) error {
    // 模拟本地数据库事务
    // 实际代码中这里会包含：
    // - 开启事务
    // - 更新订单状态
    // - 扣减余额
    // - 记录支付日志
    // - 提交或回滚事务
    return nil
}
```

---

## 错误处理

### 错误码列表

| 错误码 | 说明 | 处理建议 |
|--------|------|----------|
| `INVALID_PARAMETER` | 参数错误 | 检查请求参数 |
| `RATE_LIMITED` | 频率限制 | 降低请求频率，稍后重试 |
| `TEMPLATE_NOT_FOUND` | 模板未找到 | 检查模板ID |
| `CHANNEL_DISABLED` | 渠道被禁用 | 联系管理员 |
| `CREATE_NOTIFICATION_FAILED` | 创建通知失败 | 查看详细错误信息 |
| `NO_QUOTA` | 配额用完 | 充值或等待配额重置 |
| `SEND_NOTIFICATION_FAILED` | 发送失败 | 检查日志，可能需要重试 |

### 错误处理示例

```go
func handleError(resp *notificationpb.SendNotificationResponse) {
    if resp.ErrorCode == notificationpb.ErrorCode_ERROR_CODE_UNSPECIFIED {
        // 成功
        return
    }
    
    switch resp.ErrorCode {
    case notificationpb.ErrorCode_INVALID_PARAMETER:
        log.Printf("参数错误: %s", resp.ErrorMessage)
        // 不应重试
        
    case notificationpb.ErrorCode_RATE_LIMITED:
        log.Printf("频率限制: %s", resp.ErrorMessage)
        // 等待后重试
        time.Sleep(1 * time.Second)
        // retry...
        
    case notificationpb.ErrorCode_NO_QUOTA:
        log.Printf("配额不足: %s", resp.ErrorMessage)
        // 通知管理员充值
        alertAdmin("配额不足")
        
    case notificationpb.ErrorCode_SEND_NOTIFICATION_FAILED:
        log.Printf("发送失败: %s", resp.ErrorMessage)
        // 可以重试
        // retry with exponential backoff
        
    default:
        log.Printf("未知错误 %s: %s", resp.ErrorCode, resp.ErrorMessage)
    }
}
```

---

## 最佳实践

### 1. 选择合适的发送模式

```go
// ❌ 错误：大批量使用同步发送
for _, user := range users { // 10万用户
    client.SendNotification(ctx, req) // 同步等待，很慢
}

// ✅ 正确：大批量使用异步发送
notifications := make([]*notificationpb.Notification, len(users))
// ... 准备通知
client.BatchSendNotificationsAsync(ctx, &notificationpb.BatchSendNotificationsAsyncRequest{
    Notifications: notifications,
})
```

### 2. 设置合理的超时

```go
// 同步发送：较短超时
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
resp, err := client.SendNotification(ctx, req)

// 异步发送：较长超时（仅创建记录）
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
resp, err := client.SendNotificationAsync(ctx, req)
```

### 3. 幂等性保证

```go
// 使用唯一的 Key 保证幂等性
notification := &notificationpb.Notification{
    Key: fmt.Sprintf("order-%s-%d", orderID, time.Now().Unix()),
    // ...
}

// 重试时使用相同的 Key
// 平台会自动去重
```

### 4. 批量处理优化

```go
// 分批处理大量通知
func sendLargeNotifications(client notificationpb.NotificationServiceClient, 
    notifications []*notificationpb.Notification) error {
    
    batchSize := 100 // 每批100条
    
    for i := 0; i < len(notifications); i += batchSize {
        end := i + batchSize
        if end > len(notifications) {
            end = len(notifications)
        }
        
        batch := notifications[i:end]
        
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        _, err := client.BatchSendNotificationsAsync(ctx, 
            &notificationpb.BatchSendNotificationsAsyncRequest{
                Notifications: batch,
            })
        cancel()
        
        if err != nil {
            log.Printf("批次 %d-%d 失败: %v", i, end, err)
            continue
        }
        
        // 批次间稍作延迟，避免过载
        time.Sleep(100 * time.Millisecond)
    }
    
    return nil
}
```

### 5. 监控和日志

```go
func sendNotificationWithMonitoring(client notificationpb.NotificationServiceClient, 
    notification *notificationpb.Notification) {
    
    start := time.Now()
    ctx := withBizID(context.Background(), 12345)
    
    resp, err := client.SendNotification(ctx, &notificationpb.SendNotificationRequest{
        Notification: notification,
    })
    
    duration := time.Since(start)
    
    // 记录指标
    metrics.RecordDuration("notification.send", duration)
    
    if err != nil {
        metrics.IncrementCounter("notification.error")
        log.Printf("发送失败 [%s]: %v, 耗时: %v", notification.Key, err, duration)
        return
    }
    
    if resp.Status == notificationpb.SendStatus_SUCCEEDED {
        metrics.IncrementCounter("notification.success")
        log.Printf("发送成功 [%s]: ID=%d, 耗时: %v", 
            notification.Key, resp.NotificationId, duration)
    } else {
        metrics.IncrementCounter("notification.failed")
        log.Printf("发送失败 [%s]: %s, 耗时: %v", 
            notification.Key, resp.ErrorMessage, duration)
    }
}
```

---

## 完整示例

### 综合示例：电商订单通知

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"
    
    notificationpb "github.com/serendipityConfusion/notification-platform/api/gen/v1"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    "google.golang.org/grpc/metadata"
    "google.golang.org/protobuf/types/known/timestamppb"
)

type NotificationClient struct {
    client      notificationpb.NotificationServiceClient
    queryClient notificationpb.NotificationQueryServiceClient
    bizID       int64
}

func NewNotificationClient(addr string, bizID int64) (*NotificationClient, error) {
    conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        return nil, err
    }
    
    return &NotificationClient{
        client:      notificationpb.NewNotificationServiceClient(conn),
        queryClient: notificationpb.NewNotificationQueryServiceClient(conn),
        bizID:       bizID,
    }, nil
}

func (nc *NotificationClient) withContext() context.Context {
    md := metadata.Pairs("biz-id", fmt.Sprintf("%d", nc.bizID))
    return metadata.NewOutgoingContext(context.Background(), md)
}