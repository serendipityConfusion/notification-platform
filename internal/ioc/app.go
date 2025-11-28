package ioc

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/serendipityConfusion/notification-platform/internal/pkg/config"
	"github.com/serendipityConfusion/notification-platform/internal/pkg/registry"
	"google.golang.org/grpc"
)

// App 应用结构体
type App struct {
	GrpcServer   *grpc.Server          // gRPC 服务器
	Registry     registry.Registry     // 服务注册器（抽象接口）
	ConfigLoader config.ConfigLoader   // 配置加载器（抽象接口）
	ServiceInfo  *registry.ServiceInfo // 服务信息
}

// Run 运行应用
func (a *App) Run() error {
	// 1. 从配置加载器获取 gRPC 配置
	grpcConf := &config.GrpcConfig{}
	if err := a.ConfigLoader.Load("notification-server", grpcConf); err != nil {
		return fmt.Errorf("failed to load grpc config: %w", err)
	}

	// 2. 构造服务信息
	if a.ServiceInfo == nil {
		a.ServiceInfo = &registry.ServiceInfo{
			Name:      grpcConf.Name,
			Addr:      grpcConf.Addr,
			TTL:       10 * time.Second, // 默认 10 秒心跳
			Namespace: "/services",
		}
	} else {
		// 如果已经注入了 ServiceInfo，则更新配置中的值
		a.ServiceInfo.Name = grpcConf.Name
		a.ServiceInfo.Addr = grpcConf.Addr
	}

	// 3. 注册服务到注册中心
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := a.Registry.Register(ctx, a.ServiceInfo); err != nil {
		return fmt.Errorf("failed to register service: %w", err)
	}

	// 4. 启动 gRPC 服务器
	listener, err := net.Listen("tcp", a.ServiceInfo.Addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", a.ServiceInfo.Addr, err)
	}
	log.Printf("[App] gRPC server listening on %s", a.ServiceInfo.Addr)

	// 在 goroutine 中启动服务器
	errCh := make(chan error, 1)
	go func() {
		if err := a.GrpcServer.Serve(listener); err != nil {
			errCh <- fmt.Errorf("failed to serve: %w", err)
		}
	}()

	// 5. 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		log.Println("[App] Shutting down server...")
	case err := <-errCh:
		return err
	}

	// 6. 优雅关闭
	return a.shutdown()
}

// shutdown 优雅关闭应用
func (a *App) shutdown() error {
	log.Println("[App] Starting graceful shutdown...")

	// 1. 从注册中心注销服务
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := a.Registry.Deregister(ctx, a.ServiceInfo); err != nil {
		log.Printf("[App] Failed to deregister service: %v", err)
		// 不返回错误，继续关闭流程
	}

	// 2. 关闭注册器
	if err := a.Registry.Close(); err != nil {
		log.Printf("[App] Failed to close registry: %v", err)
	}

	// 3. 优雅停止 gRPC 服务器
	a.GrpcServer.GracefulStop()
	log.Println("[App] Server stopped gracefully")

	return nil
}

// GetServiceInfo 获取服务信息
func (a *App) GetServiceInfo() *registry.ServiceInfo {
	return a.ServiceInfo
}

// SetServiceMetadata 设置服务元数据
func (a *App) SetServiceMetadata(metadata map[string]string) {
	if a.ServiceInfo == nil {
		a.ServiceInfo = &registry.ServiceInfo{}
	}
	a.ServiceInfo.Metadata = metadata
}
