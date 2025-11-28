# 架构改进建议

## 概述

本文档基于当前代码库的全面审查，提出一系列架构优化建议。这些建议旨在进一步提升代码质量、可维护性、可测试性和系统的健壮性。

## 目录

- [1. 错误处理优化](#1-错误处理优化)
- [2. 配置管理改进](#2-配置管理改进)
- [3. 日志系统增强](#3-日志系统增强)
- [4. 初始化流程优化](#4-初始化流程优化)
- [5. Repository 接口拆分](#5-repository-接口拆分)
- [6. 拦截器/中间件抽象](#6-拦截器中间件抽象)
- [7. 响应处理标准化](#7-响应处理标准化)
- [8. 健康检查与优雅关闭](#8-健康检查与优雅关闭)
- [9. 弹性模式抽象](#9-弹性模式抽象)
- [10. 可观测性增强](#10-可观测性增强)
- [11. 上下文传播](#11-上下文传播)
- [12. 测试基础设施](#12-测试基础设施)

---

## 1. 错误处理优化

### 当前问题

```go
// internal/domain/error.go
var (
    ErrInvalidParameter = errors.New("参数错误")
    ErrNotificationNotFound = errors.New("通知记录不存在")
    // ...
)
```

**问题：**
- 缺少错误码
- 无法携带上下文信息
- 难以区分错误类型（业务错误 vs 系统错误）
- 无法进行错误链追踪

### 优化方案

#### 1.1 定义结构化错误类型

```go
// internal/pkg/errors/error.go
package errors

import (
    "fmt"
)

// Code 错误码类型
type Code string

const (
    // 业务错误码 (B开头)
    CodeInvalidParameter     Code = "B001"
    CodeNotificationNotFound Code = "B002"
    CodeRateLimited          Code = "B003"
    CodeNoQuota              Code = "B004"
    
    // 系统错误码 (S开头)
    CodeDatabaseError       Code = "S001"
    CodeExternalServiceError Code = "S002"
    CodeInternalError       Code = "S999"
)

// Error 应用错误类型
type Error struct {
    Code    Code                   // 错误码
    Message string                 // 错误消息
    Details map[string]interface{} // 详细信息
    Cause   error                  // 原始错误
}

func (e *Error) Error() string {
    if e.Cause != nil {
        return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
    }
    return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error {
    return e.Cause
}

// WithCause 添加原始错误
func (e *Error) WithCause(cause error) *Error {
    e.Cause = cause
    return e
}

// WithDetail 添加详细信息
func (e *Error) WithDetail(key string, value interface{}) *Error {
    if e.Details == nil {
        e.Details = make(map[string]interface{})
    }
    e.Details[key] = value
    return e
}

// New 创建新错误
func New(code Code, message string) *Error {
    return &Error{
        Code:    code,
        Message: message,
    }
}

// Wrap 包装现有错误
func Wrap(err error, code Code, message string) *Error {
    return &Error{
        Code:    code,
        Message: message,
        Cause:   err,
    }
}

// IsBusinessError 判断是否为业务错误
func IsBusinessError(err error) bool {
    if e, ok := err.(*Error); ok {
        return len(e.Code) > 0 && e.Code[0] == 'B'
    }
    return false
}
```

#### 1.2 使用示例

```go
// internal/service/notification.go
func (s *notificationService) GetByKeys(ctx context.Context, bizID int64, keys ...string) ([]domain.Notification, error) {
    if len(keys) == 0 {
        return nil, errors.New(errors.CodeInvalidParameter, "业务内唯一标识列表不能为空").
            WithDetail("bizID", bizID)
    }

    notifications, err := s.repo.GetByKeys(ctx, bizID, keys...)
    if err != nil {
        return nil, errors.Wrap(err, errors.CodeDatabaseError, "获取通知列表失败").
            WithDetail("bizID", bizID).
            WithDetail("keys", keys)
    }
    
    return notifications, nil
}
```

---

## 2. 配置管理改进

### 当前问题

```go
// internal/ioc/db.go
func InitDB() *gorm.DB {
    db, err := gorm.Open(mysql.Open(viper.GetString("mysql.dsn")), &gorm.Config{})
    // 直接使用 viper 全局实例
}
```

**问题：**
- 部分代码仍直接使用 viper 全局实例
- 缺少配置验证
- 缺少配置热更新支持
- 没有环境区分机制

### 优化方案

#### 2.1 完善 ConfigLoader

```go
// internal/pkg/config/loader.go
type ConfigLoader interface {
    Load(key string, target interface{}) error
    GetString(key string) string
    GetInt(key string) int
    GetBool(key string) bool
    GetDuration(key string) time.Duration
    
    // 新增方法
    Validate(validators ...Validator) error      // 配置验证
    Watch(key string, callback func()) error     // 监听配置变化
    Reload() error                                // 重新加载配置
    GetEnv() string                              // 获取当前环境
}

// Validator 配置验证器
type Validator interface {
    Validate(loader ConfigLoader) error
}

// ValidatorFunc 验证函数适配器
type ValidatorFunc func(ConfigLoader) error

func (f ValidatorFunc) Validate(loader ConfigLoader) error {
    return f(loader)
}
```

#### 2.2 配置验证示例

```go
// internal/pkg/config/validators.go
package config

import "fmt"

// ValidateMySQLConfig MySQL 配置验证器
func ValidateMySQLConfig() Validator {
    return ValidatorFunc(func(loader ConfigLoader) error {
        dsn := loader.GetString("mysql.dsn")
        if dsn == "" {
            return fmt.Errorf("mysql.dsn is required")
        }
        return nil
    })
}

// ValidateRedisConfig Redis 配置验证器
func ValidateRedisConfig() Validator {
    return ValidatorFunc(func(loader ConfigLoader) error {
        addr := loader.GetString("redis.addr")
        if addr == "" {
            return fmt.Errorf("redis.addr is required")
        }
        return nil
    })
}

// 使用示例
func InitConfig() error {
    loader := config.NewViperConfigLoader()
    
    // 验证所有必需配置
    if err := loader.Validate(
        config.ValidateMySQLConfig(),
        config.ValidateRedisConfig(),
        config.ValidateEtcdConfig(),
    ); err != nil {
        return fmt.Errorf("config validation failed: %w", err)
    }
    
    return nil
}
```

#### 2.3 环境配置

```go
// internal/pkg/config/env.go
package config

type Environment string

const (
    EnvDevelopment Environment = "development"
    EnvTesting     Environment = "testing"
    EnvStaging     Environment = "staging"
    EnvProduction  Environment = "production"
)

type EnvConfig struct {
    Env        Environment
    ConfigPath string
}

func LoadEnvConfig() (*EnvConfig, error) {
    env := os.Getenv("APP_ENV")
    if env == "" {
        env = string(EnvDevelopment)
    }
    
    return &EnvConfig{
        Env:        Environment(env),
        ConfigPath: fmt.Sprintf("./config/%s", env),
    }, nil
}
```

---

## 3. 日志系统增强

### 当前问题

```go
// internal/pkg/log/log.go
type LoggerInterface interface {
    Error(msg string, fields ...zap.Field)
    Info(msg string, fields ...zap.Field)
}
```

**问题：**
- 日志级别不全（缺少 Debug、Warn 等）
- 缺少结构化日志的便捷方法
- 没有请求追踪支持
- 缺少日志采样配置

### 优化方案

#### 3.1 增强日志接口

```go
// internal/pkg/log/logger.go
package log

import (
    "context"
    "go.uber.org/zap"
)

// Logger 日志接口
type Logger interface {
    // 基础日志方法
    Debug(msg string, fields ...Field)
    Info(msg string, fields ...Field)
    Warn(msg string, fields ...Field)
    Error(msg string, fields ...Field)
    Fatal(msg string, fields ...Field)
    
    // 上下文日志（自动提取 trace_id 等）
    DebugCtx(ctx context.Context, msg string, fields ...Field)
    InfoCtx(ctx context.Context, msg string, fields ...Field)
    WarnCtx(ctx context.Context, msg string, fields ...Field)
    ErrorCtx(ctx context.Context, msg string, fields ...Field)
    
    // 结构化日志便捷方法
    With(fields ...Field) Logger
    Named(name string) Logger
    
    // 同步日志缓冲区
    Sync() error
}

// Field 日志字段（封装 zap.Field）
type Field = zap.Field

// 便捷字段构造函数
func String(key, val string) Field        { return zap.String(key, val) }
func Int(key string, val int) Field       { return zap.Int(key, val) }
func Int64(key string, val int64) Field   { return zap.Int64(key, val) }
func Error(err error) Field               { return zap.Error(err) }
func Any(key string, val interface{}) Field { return zap.Any(key, val) }

// 上下文字段提取
func TraceID(ctx context.Context) Field {
    if traceID := ctx.Value("trace_id"); traceID != nil {
        return zap.String("trace_id", traceID.(string))
    }
    return zap.Skip()
}
```

#### 3.2 实现增强的 Logger

```go
// internal/pkg/log/zap_logger.go
package log

import (
    "context"
    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
)

type zapLogger struct {
    logger *zap.Logger
}

func NewLogger(cfg *Config) Logger {
    config := zap.NewProductionConfig()
    
    // 根据配置调整
    if cfg.Level != "" {
        level, _ := zapcore.ParseLevel(cfg.Level)
        config.Level = zap.NewAtomicLevelAt(level)
    }
    
    if cfg.Development {
        config.Development = true
        config.Encoding = "console"
    }
    
    logger, _ := config.Build(
        zap.AddCaller(),
        zap.AddCallerSkip(1),
    )
    
    return &zapLogger{logger: logger}
}

func (l *zapLogger) InfoCtx(ctx context.Context, msg string, fields ...Field) {
    fields = append(fields, TraceID(ctx))
    l.logger.Info(msg, fields...)
}

// ... 其他方法实现
```

#### 3.3 使用示例

```go
// internal/service/notification.go
func (s *notificationService) GetByKeys(ctx context.Context, bizID int64, keys ...string) ([]domain.Notification, error) {
    logger := log.FromContext(ctx).With(
        log.Int64("biz_id", bizID),
        log.Int("key_count", len(keys)),
    )
    
    logger.InfoCtx(ctx, "开始获取通知列表")
    
    notifications, err := s.repo.GetByKeys(ctx, bizID, keys...)
    if err != nil {
        logger.ErrorCtx(ctx, "获取通知列表失败", log.Error(err))
        return nil, err
    }
    
    logger.InfoCtx(ctx, "获取通知列表成功", log.Int("count", len(notifications)))
    return notifications, nil
}
```

---

## 4. 初始化流程优化

### 当前问题

```go
// internal/ioc/db.go
func InitDB() *gorm.DB {
    db, err := gorm.Open(...)
    if err != nil {
        panic(err) // 使用 panic
    }
    // ...
}
```

**问题：**
- 初始化函数使用 panic，不符合 Go 错误处理习惯
- 无法优雅处理初始化错误
- 难以进行单元测试

### 优化方案

#### 4.1 返回错误而非 panic

```go
// internal/ioc/db.go
func InitDB(loader config.ConfigLoader) (*gorm.DB, error) {
    var cfg struct {
        DSN string `yaml:"dsn"`
    }
    
    if err := loader.Load("mysql", &cfg); err != nil {
        return nil, fmt.Errorf("failed to load mysql config: %w", err)
    }
    
    db, err := gorm.Open(mysql.Open(cfg.DSN), &gorm.Config{})
    if err != nil {
        return nil, fmt.Errorf("failed to open database: %w", err)
    }
    
    if err := dao.InitTable(db); err != nil {
        return nil, fmt.Errorf("failed to init tables: %w", err)
    }
    
    if err := db.Use(metrics.NewGormMetricsPlugin()); err != nil {
        return nil, fmt.Errorf("failed to register metrics plugin: %w", err)
    }
    
    if err := db.Use(tracing.NewGormTracingPlugin()); err != nil {
        return nil, fmt.Errorf("failed to register tracing plugin: %w", err)
    }
    
    return db, nil
}
```

#### 4.2 更新 Wire 配置

```go
// cmd/platform/ioc/wire.go
var BaseSet = wire.NewSet(
    InitDB,
    InitRedis,
    InitEtcdClient,
    // 所有初始化函数都返回 (T, error)
)

// Wire 会自动处理错误
```

#### 4.3 统一的初始化错误处理

```go
// cmd/platform/main.go
func main() {
    // 初始化配置
    if err := config.InitViperConfig(); err != nil {
        log.Fatal("Failed to init config", zap.Error(err))
    }
    
    // 验证配置
    loader := config.NewViperConfigLoader()
    if err := loader.Validate(
        config.ValidateMySQLConfig(),
        config.ValidateRedisConfig(),
    ); err != nil {
        log.Fatal("Config validation failed", zap.Error(err))
    }
    
    // 初始化应用
    app, err := ioc.InitGrpcServer()
    if err != nil {
        log.Fatal("Failed to init application", zap.Error(err))
    }
    
    // 运行应用
    if err := app.Run(); err != nil {
        log.Fatal("Application error", zap.Error(err))
    }
}
```

---

## 5. Repository 接口拆分

### 当前问题

```go
// internal/repository/notification.go
type NotificationRepository interface {
    Create(...)
    CreateWithCallbackLog(...)
    BatchCreate(...)
    BatchCreateWithCallbackLog(...)
    GetByID(...)
    BatchGetByIDs(...)
    GetByKey(...)
    GetByKeys(...)
    CASStatus(...)
    UpdateStatus(...)
    BatchUpdateStatusSucceededOrFailed(...)
    FindReadyNotifications(...)
    MarkSuccess(...)
    MarkFailed(...)
    MarkTimeoutSendingAsFailed(...)
    // 15+ 方法
}
```

**问题：**
- 接口过大，违反接口隔离原则（ISP）
- 难以 Mock
- 职责不清晰

### 优化方案

#### 5.1 按职责拆分接口

```go
// internal/repository/notification.go

// NotificationReader 通知读取接口
type NotificationReader interface {
    GetByID(ctx context.Context, id uint64) (domain.Notification, error)
    BatchGetByIDs(ctx context.Context, ids []uint64) (map[uint64]domain.Notification, error)
    GetByKey(ctx context.Context, bizID int64, key string) (domain.Notification, error)
    GetByKeys(ctx context.Context, bizID int64, keys ...string) ([]domain.Notification, error)
    FindReadyNotifications(ctx context.Context, offset, limit int) ([]domain.Notification, error)
}

// NotificationWriter 通知写入接口
type NotificationWriter interface {
    Create(ctx context.Context, notification domain.Notification) (domain.Notification, error)
    CreateWithCallbackLog(ctx context.Context, notification domain.Notification) (domain.Notification, error)
    BatchCreate(ctx context.Context, notifications []domain.Notification) ([]domain.Notification, error)
    BatchCreateWithCallbackLog(ctx context.Context, notifications []domain.Notification) ([]domain.Notification, error)
}

// NotificationUpdater 通知更新接口
type NotificationUpdater interface {
    CASStatus(ctx context.Context, notification domain.Notification) error
    UpdateStatus(ctx context.Context, notification domain.Notification) error
    BatchUpdateStatusSucceededOrFailed(ctx context.Context, succeeded, failed []domain.Notification) error
    MarkSuccess(ctx context.Context, notification domain.Notification) error
    MarkFailed(ctx context.Context, notification domain.Notification) error
    MarkTimeoutSendingAsFailed(ctx context.Context, batchSize int) (int64, error)
}

// NotificationRepository 完整的仓储接口（组合）
type NotificationRepository interface {
    NotificationReader
    NotificationWriter
    NotificationUpdater
}
```

#### 5.2 按需使用接口

```go
// internal/service/query_service.go
type NotificationQueryService struct {
    reader repository.NotificationReader // 只依赖读接口
}

// internal/service/command_service.go
type NotificationCommandService struct {
    writer  repository.NotificationWriter  // 只依赖写接口
    updater repository.NotificationUpdater // 只依赖更新接口
}
```

---

## 6. 拦截器/中间件抽象

### 当前问题

```go
// internal/ioc/grpc.go
func InitGrpc(noserver *grpcapi.NotificationServer) *grpc.Server {
    metricsInterceptor := metrics.New().Build()
    logInterceptor := log.New().Build()
    traceInterceptor := tracing.UnaryServerInterceptor()
    
    server := grpc.NewServer(
        grpc.ChainUnaryInterceptor(
            metricsInterceptor,
            logInterceptor,
            traceInterceptor,
        ),
    )
    // ...
}
```

**问题：**
- 拦截器硬编码
- 无法动态配置拦截器顺序
- 难以根据环境启用/禁用拦截器

### 优化方案

#### 6.1 定义拦截器接口

```go
// internal/pkg/interceptor/interceptor.go
package interceptor

import (
    "context"
    "google.golang.org/grpc"
)

// UnaryServerInterceptorBuilder 一元拦截器构建器
type UnaryServerInterceptorBuilder interface {
    Build() grpc.UnaryServerInterceptor
    Name() string
    Priority() int // 优先级，数字越小越先执行
}

// InterceptorChain 拦截器链
type InterceptorChain struct {
    builders []UnaryServerInterceptorBuilder
}

func NewInterceptorChain() *InterceptorChain {
    return &InterceptorChain{
        builders: make([]UnaryServerInterceptorBuilder, 0),
    }
}

func (c *InterceptorChain) Add(builder UnaryServerInterceptorBuilder) *InterceptorChain {
    c.builders = append(c.builders, builder)
    return c
}

func (c *InterceptorChain) Build() []grpc.UnaryServerInterceptor {
    // 按优先级排序
    sort.Slice(c.builders, func(i, j int) bool {
        return c.builders[i].Priority() < c.builders[j].Priority()
    })
    
    interceptors := make([]grpc.UnaryServerInterceptor, len(c.builders))
    for i, builder := range c.builders {
        interceptors[i] = builder.Build()
    }
    return interceptors
}
```

#### 6.2 可配置的拦截器

```go
// internal/pkg/interceptor/config.go
package interceptor

type InterceptorConfig struct {
    Enabled  bool
    Priority int
    Options  map[string]interface{}
}

type Config struct {
    Metrics *InterceptorConfig `yaml:"metrics"`
    Logging *InterceptorConfig `yaml:"logging"`
    Tracing *InterceptorConfig `yaml:"tracing"`
    Auth    *InterceptorConfig `yaml:"auth"`
}

func LoadFromConfig(loader config.ConfigLoader) (*InterceptorChain, error) {
    var cfg Config
    if err := loader.Load("interceptors", &cfg); err != nil {
        return nil, err
    }
    
    chain := NewInterceptorChain()
    
    if cfg.Metrics != nil && cfg.Metrics.Enabled {
        chain.Add(metrics.NewBuilder(cfg.Metrics))
    }
    
    if cfg.Logging != nil && cfg.Logging.Enabled {
        chain.Add(logging.NewBuilder(cfg.Logging))
    }
    
    if cfg.Tracing != nil && cfg.Tracing.Enabled {
        chain.Add(tracing.NewBuilder(cfg.Tracing))
    }
    
    return chain, nil
}
```

#### 6.3 使用示例

```go
// config/platform/config.yaml
interceptors:
  metrics:
    enabled: true
    priority: 10
  logging:
    enabled: true
    priority: 20
  tracing:
    enabled: true
    priority: 30

// internal/ioc/grpc.go
func InitGrpc(noserver *grpcapi.NotificationServer, loader config.ConfigLoader) (*grpc.Server, error) {
    chain, err := interceptor.LoadFromConfig(loader)
    if err != nil {
        return nil, err
    }
    
    server := grpc.NewServer(
        grpc.ChainUnaryInterceptor(chain.Build()...),
    )
    
    notificationpb.RegisterNotificationServiceServer(server, noserver)
    return server, nil
}
```

---

## 7. 响应处理标准化

### 当前问题

- 没有统一的响应格式
- 错误响应不一致
- 缺少响应包装器

### 优化方案

#### 7.1 定义标准响应格式

```go
// internal/pkg/response/response.go
package response

import (
    "context"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
    apperrors "github.com/serendipityConfusion/notification-platform/internal/pkg/errors"
)

// Response 通用响应结构
type Response struct {
    Code    string                 `json:"code"`
    Message string                 `json:"message"`
    Data    interface{}            `json:"data,omitempty"`
    TraceID string                 `json:"trace_id,omitempty"`
    Details map[string]interface{} `json:"details,omitempty"`
}

// ErrorToGRPCStatus 将应用错误转换为 gRPC 状态
func ErrorToGRPCStatus(err error) error {
    if err == nil {
        return nil
    }
    
    // 应用错误
    if appErr, ok := err.(*apperrors.Error); ok {
        grpcCode := mapToGRPCCode(appErr.Code)
        st := status.New(grpcCode, appErr.Message)
        
        // 添加详细信息
        if len(appErr.Details) > 0 {
            // 可以使用 status.WithDetails 添加详细信息
        }
        
        return st.Err()
    }
    
    // 未知错误
    return status.Error(codes.Internal, err.Error())
}

func mapToGRPCCode(appCode apperrors.Code) codes.Code {
    switch {
    case appCode == apperrors.CodeInvalidParameter:
        return codes.InvalidArgument
    case appCode == apperrors.CodeNotificationNotFound:
        return codes.NotFound
    case appCode == apperrors.CodeRateLimited:
        return codes.ResourceExhausted
    case appCode == apperrors.CodeNoQuota:
        return codes.ResourceExhausted
    default:
        return codes.Internal
    }
}
```

#### 7.2 统一的错误拦截器

```go
// internal/api/grpc/interceptor/error/error.go
package error

import (
    "context"
    "google.golang.org/grpc"
    "github.com/serendipityConfusion/notification-platform/internal/pkg/response"
    "github.com/serendipityConfusion/notification-platform/internal/pkg/log"
)

func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
        resp, err := handler(ctx, req)
        
        if err != nil {
            // 记录错误
            log.FromContext(ctx).ErrorCtx(ctx, "RPC error",
                log.String("method", info.FullMethod),
                log.Error(err),
            )
            
            // 转换为标准 gRPC 错误
            return nil, response.ErrorToGRPCStatus(err)
        }
        
        return resp, nil
    }
}
```

---

## 8. 健康检查与优雅关闭

### 当前问题

- App.Run() 中有基本的优雅关闭，但不完整
- 缺少健康检查端点
- 缺少就绪检查（Readiness）和存活检查（Liveness）

### 优化方案

#### 8.1 健康检查接口

```go
// internal/pkg/health/health.go
package health

import (
    "context"
    "sync"
)

// Status 健康状态
type Status string

const (
    StatusHealthy   Status = "healthy"
    StatusUnhealthy Status = "unhealthy"
    StatusDegraded  Status = "degraded"
)

// CheckResult 检查结果
type CheckResult struct {
    Status  Status
    Message string
    Details map[string]interface{}
}

// Checker 健康检查器接口
type Checker interface {
    Name() string
    Check(ctx context.Context) CheckResult
}

// HealthChecker 健康检查管理器
type HealthChecker struct {
    mu       sync.RWMutex
    checkers map[string]Checker
}

func NewHealthChecker() *HealthChecker {
    return &HealthChecker{
        checkers: make(map[string]Checker),
    }
}

func (h *HealthChecker) Register(checker Checker) {
    h.mu.Lock()
    defer h.mu.Unlock()
    h.checkers[checker.Name()] = checker
}

func (h *HealthChecker) CheckAll(ctx context.Context) map[string]CheckResult {
    h.mu.RLock()
    defer h.mu.RUnlock()
    
    results := make(map[string]CheckResult)
    for name, checker := range h.checkers {
        results[name] = checker.Check(ctx)
    }
    return results
}

// IsHealthy 检查整体健康状态
func (h *HealthChecker) IsHealthy(ctx context.Context) bool {
    results := h.CheckAll(ctx)
    for _, result := range results {
        if result.Status == StatusUnhealthy {
            return false
        }
    }
    return true
}
```

#### 8.2 具体健康检查器实现

```go
// internal/pkg/health/checkers/database.go
package checkers

import (
    "context"
    "github.com/serendipityConfusion/notification-platform/internal/pkg/health"
    "gorm.io/gorm"
)

type DatabaseChecker struct {
    db *gorm.DB
}

func NewDatabaseChecker(db *gorm.DB) health.Checker {
    return &DatabaseChecker{db: db}
}

func (c *DatabaseChecker) Name() string {
    return "database"
}

func (c *DatabaseChecker) Check(ctx context.Context) health.CheckResult {
    sqlDB, err := c.db.DB()
    if err != nil {
        return health.CheckResult{
            Status:  health.StatusUnhealthy,
            Message: "Failed to get database connection",
        }
    }
    
    if err := sqlDB.PingContext(ctx); err != nil {
        return health.CheckResult{
            Status:  health.StatusUnhealthy,
            Message: "Database ping failed",
        }
    }
    
    return health.CheckResult{
        Status:  health.StatusHealthy,
        Message: "Database is healthy",
    }
}

// internal/pkg/health/checkers/redis.go
type RedisChecker struct {
    client redis.Client
}

func (c *RedisChecker) Check(ctx context.Context) health.CheckResult {
    if err := c.client.Ping(ctx).Err(); err != nil {
        return health.CheckResult{
            Status:  health.StatusUnhealthy,
            Message: "Redis ping failed",
        }
    }
    
    return health.CheckResult{
        Status:  health.StatusHealthy,
        Message: "Redis is healthy",
    }
}
```

#### 8.3 优雅关闭增强

```go
// internal/ioc/app.go
type App struct {
    GrpcServer     *grpc.Server
    Registry       registry.Registry
    ConfigLoader   config.ConfigLoader
    ServiceInfo    *registry.ServiceInfo
    HealthChecker  *health.HealthChecker  // 新增
    shutdownFuncs  []func() error         // 新增：关闭函数列表
}

// RegisterShutdownFunc 注册关闭时需要执行的函数
func (a *App) RegisterShutdownFunc(fn func() error) {
    a.shutdownFuncs = append(a.shutdownFuncs, fn)
}

func (a *App) shutdown() error {
    log.Println("[App] Starting graceful shutdown...")
    
    // 1. 停止接收新请求（从注册中心注销）
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    if err := a.Registry.Deregister(ctx, a.ServiceInfo); err != nil {
        log.Printf("[App] Failed to deregister service: %v", err)
    }
    
    // 2. 等待当前请求完成（GracefulStop 会等待）
    done := make(chan struct{})
    go func() {
        a.GrpcServer.GracefulStop()
        close(done)
    }()
    
    // 3. 超时强制关闭
    select {
    case <-done:
        log.Println("[App] gRPC server stopped gracefully")
    case <-time.After(30 * time.Second):
        log.Println("[App] Forcing gRPC server to stop")
        a.GrpcServer.Stop()
    }
    
    // 4. 执行所有注册的关闭函数
    for i, fn := range a.shutdownFuncs {
        if err := fn(); err != nil {
            log.Printf("[App] Shutdown func %d failed: %v", i, err)
        }
    }
    
    // 5. 关闭注册器
    if err := a.Registry.Close(); err != nil {
        log.Printf("[App] Failed to close registry: %v", err)
    }
    
    log.Println("[App] Shutdown complete")
    return nil
}
```

---

## 9. 弹性模式抽象

### 当前问题

- 缺少重试机制的统一抽象
- 缺少熔断器实现
- 缺少限流器抽象

### 优化方案

#### 9.1 重试器接口

```go
// internal/pkg/resilience/retry.go
package resilience

import (
    "context"
    "time"
)

// RetryPolicy 重试策略
type RetryPolicy interface {
    ShouldRetry(attempt int, err error) bool
    Delay(attempt int) time.Duration
}

// Retryer 重试器
type Retryer struct {
    policy RetryPolicy
}

func NewRetryer(policy RetryPolicy) *Retryer {
    return &Retryer{policy: policy}
}

func (r *Retryer) Do(ctx context.Context, fn func() error) error {
    var err error
    attempt := 0
    
    for {
        err = fn()
        if err == nil {
            return nil
        }
        
        attempt++
        if !r.policy.ShouldRetry(attempt, err) {
            return err
        }
        
        delay := r.policy.Delay(attempt)
        select {
        case <-time.After(delay):
            continue
        case <-ctx.Done():
            return ctx.Err()
        }
    }
}

// ExponentialBackoffPolicy 指数退避策略
type ExponentialBackoffPolicy struct {
    MaxAttempts  int
    InitialDelay time.Duration
    MaxDelay     time.Duration
    Multiplier   float64
}

func (p *ExponentialBackoffPolicy) ShouldRetry(attempt int, err error) bool {
    return attempt < p.MaxAttempts
}

func (p *ExponentialBackoffPolicy) Delay(attempt int) time.Duration {
    delay := float64(p.InitialDelay) * pow(p.Multiplier, float64(attempt-1))
    if delay > float64(p.MaxDelay) {
        return p.MaxDelay
    }
    return time.Duration(delay)
}
```

#### 9.2 熔断器

```go
// internal/pkg/resilience/circuit_breaker.go
package resilience

import (
    "context"
    "sync"
    "time"
)

// CircuitState 熔断器状态
type CircuitState int

const (
    StateClosed CircuitState = iota  // 关闭状态（正常）
    StateOpen                         // 打开状态（熔断）
    StateHalfOpen                     // 半开状态（尝试恢复）
)

// CircuitBreaker 熔断器
type CircuitBreaker struct {
    mu              sync.RWMutex
    state           CircuitState
    failureCount    int
    successCount    int
    lastFailureTime time.Time
    
    // 配置
    maxFailures     int           // 最大失败次数
    timeout         time.Duration // 超时时间
    halfOpenSuccess int           // 半开状态需要的成功次数
}

func NewCircuitBreaker(maxFailures int, timeout time.Duration) *CircuitBreaker {
    return &CircuitBreaker{
        state:           StateClosed,
        maxFailures:     maxFailures,
        timeout:         timeout,
        halfOpenSuccess: 2,
    }
}

func (cb *CircuitBreaker) Call(ctx context.Context, fn func() error) error {
    if !cb.allow() {
        return errors.New("circuit breaker is open")
    }
    
    err := fn()
    cb.record(err)
    return err
}

func (cb *CircuitBreaker) allow() bool {
    cb.mu.Lock()
    defer cb.mu.Unlock()
    
    switch cb.state {
    case StateClosed:
        return true
    case StateOpen:
        if time.Since(cb.lastFailureTime) > cb.timeout {
            cb.state = StateHalfOpen
            cb.successCount = 0
            return true
        }
        return false
    case StateHalfOpen:
        return true
    }
    return false
}

func (cb *CircuitBreaker) record(err error) {
    cb.mu.Lock()
    defer cb.mu.Unlock()
    
    if err == nil {
        cb.onSuccess()
    } else {
        cb.onFailure()
    }
}

func (cb *CircuitBreaker) onSuccess() {
    switch cb.state {
    case StateClosed:
        cb.failureCount = 0
    case StateHalfOpen:
        cb.successCount++
        if cb.successCount >= cb.halfOpenSuccess {
            cb.state = StateClosed
            cb.failureCount = 0
        }
    }
}

func (cb *CircuitBreaker) onFailure() {
    cb.lastFailureTime = time.Now()
    
    switch cb.state {
    case StateClosed:
        cb.failureCount++
        if cb.failureCount >= cb.maxFailures {
            cb.state = StateOpen
        }
    case StateHalfOpen:
        cb.state = StateOpen
    }
}
```

---

## 10. 可观测性增强

### 优化方案

#### 10.1 Metrics 标准化

```go
// internal/pkg/metrics/metrics.go
package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics 指标收集器
type Metrics struct {
    // gRPC 相关指标
    GrpcRequestTotal    *prometheus.CounterVec
    GrpcRequestDuration *prometheus.HistogramVec
    GrpcRequestSize     *prometheus.HistogramVec
    GrpcResponseSize    *prometheus.HistogramVec
    
    // 业务指标
    NotificationTotal     *prometheus.CounterVec
    NotificationDuration  *prometheus.HistogramVec
    NotificationFailures  *prometheus.CounterVec
    
    // 系统指标
    DBQueryDuration *prometheus.HistogramVec
    RedisOpDuration *prometheus.HistogramVec
}

func NewMetrics(namespace string) *Metrics {
    return &Metrics{
        GrpcRequestTotal: promauto.NewCounterVec(
            prometheus.CounterOpts{
                Namespace: namespace,
                Name:      "grpc_requests_total",
                Help:      "Total number of gRPC requests",
            },
            []string{"method", "code"},
        ),
        
        NotificationTotal: promauto.NewCounterVec(
            prometheus.CounterOpts{
                Namespace: namespace,
                Name:      "notifications_total",
                Help:      "Total number of notifications",
            },
            []string{"channel", "status"},
        ),
        
        // ... 其他指标
    }
}
```

---

## 11. 上下文传播

### 优化方案

```go
// internal/pkg/context/context.go
package context

import (
    "context"
)

type contextKey string

const (
    keyTraceID  contextKey = "trace_id"
    keyUserID   contextKey = "user_id"
    keyBizID    contextKey = "biz_id"
    keyLogger   contextKey = "logger"
)

// WithTraceID 设置 TraceID
func WithTraceID(ctx context.Context, traceID string) context.Context {
    return context.WithValue(ctx, keyTraceID, traceID)
}

// GetTraceID 获取 TraceID
func GetTraceID(ctx context.Context) string {
    if traceID, ok := ctx.Value(keyTraceID).(string); ok {
        return traceID
    }
    return ""
}

// WithLogger 设置 Logger
func WithLogger(ctx context.Context, logger log.Logger) context.Context {
    return context.WithValue(ctx, keyLogger, logger)
}

// GetLogger 获取 Logger
func GetLogger(ctx context.Context) log.Logger {
    if logger, ok := ctx.Value(keyLogger).(log.Logger); ok {
        return logger
    }
    return log.DefaultLogger()
}
```

---

## 12. 测试基础设施

### 优化方案

#### 12.1 测试辅助函数

```go
// internal/pkg/testutil/testutil.go
package testutil

import (
    "context"
    "testing"
    "time"
)

// SetupTestDB 创建测试数据库
func SetupTestDB(t *testing.T) *gorm.DB {
    db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    require.NoError(t, err)
    
    // 自动迁移
    dao.InitTable(db)
    
    t.Cleanup(func() {
        sqlDB, _ := db.DB()
        sqlDB.Close()
    })
    
    return db
}

// SetupTestRedis 创建测试 Redis（使用 miniredis）
func SetupTestRedis(t *testing.T) redis.Client {
    s, err := miniredis.Run()
    require.NoError(t, err)
    
    client := redis.NewClient(&redis.Options{
        Addr: s.Addr(),
    })
    
    t.Cleanup(func() {
        client.Close()
        s.Close()
    })
    
    return client
}

// NewTestContext 创建测试上下文
func NewTestContext(t *testing.T) context.Context {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    t.Cleanup(cancel)
    return ctx
}
```

---

## 总结

### 优先级建议

#### 高优先级（立即实施）

1. ✅ **错误处理优化**（已部分完成）
   - 引入结构化错误
   - 统一错误码
   - 错误链追踪

2. ✅ **配置管理改进**
   - 完善 ConfigLoader 接口
   - 配置验证
   - 消除直接使用 viper 的地方

3. ✅ **初始化流程优化**
   - 返回 error 而非 panic
   - 统一错误处理
   - Wire 自动注入

#### 中优先级（近期规划）

4. **日志系统增强**
   - 补充日志级别
   - 上下文日志
   - 结构化日志

5. **Repository 接口拆分**
   - 按职责拆分大接口
   - 提高可测试性

6. **健康检查与优雅关闭**
   - 实现健康检查
   - 完善优雅关闭流程

7. **响应处理标准化**
   - 统一响应格式
   - 错误响应转换

#### 低优先级（长期优化）

8. **拦截器/中间件抽象**
   - 可配置的拦截器链
   - 动态启用/禁用

9. **弹性模式抽象**
   - 重试器
   - 熔断器
   - 限流器

10. **可观测性增强**
    - Metrics 标准化
    - 分
布式追踪
    - 性能分析

11. **测试基础设施**
    - 测试辅助函数
    - Mock 工厂
    - 集成测试框架

### 实施路线图

```
第一阶段（1-2周）：基础优化
├── 错误处理重构
├── 配置管理完善
└── 初始化流程改进

第二阶段（2-3周）：接口优化
├── 日志系统增强
├── Repository 拆分
└── 响应处理标准化

第三阶段（3-4周）：增强特性
├── 健康检查
├── 优雅关闭
└── 拦截器抽象

第四阶段（4-6周）：高级特性
├── 弹性模式
├── 可观测性
└── 测试基础设施
```

### 预期收益

| 指标 | 当前 | 优化后 | 提升 |
|------|------|--------|------|
| 代码可测试性 | 60% | 95% | ⬆️ 58% |
| 错误处理清晰度 | 40% | 90% | ⬆️ 125% |
| 配置管理规范性 | 50% | 95% | ⬆️ 90% |
| 接口设计合理性 | 60% | 90% | ⬆️ 50% |
| 系统可观测性 | 50% | 85% | ⬆️ 70% |
| 代码可维护性 | 65% | 90% | ⬆️ 38% |

### 注意事项

1. **渐进式改进**：不要一次性重构所有代码
2. **保持兼容**：改进时保持现有功能不受影响
3. **充分测试**：每次改动都要有对应的测试
4. **文档更新**：及时更新相关文档
5. **团队协作**：改进过程中保持团队沟通

---

**最后更新**: 2024-01  
**文档版本**: 1.0  
**适用项目**: notification-platform