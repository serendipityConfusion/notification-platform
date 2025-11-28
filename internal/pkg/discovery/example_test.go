package discovery_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/serendipityConfusion/notification-platform/internal/pkg/discovery"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// Example_getService 演示如何获取单个服务地址
func Example_getService() {
	// 创建 etcd 客户端
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"localhost:2379"},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// 创建服务发现客户端
	sd := discovery.NewServiceDiscovery(client)

	// 获取服务地址
	ctx := context.Background()
	addr, err := sd.GetService(ctx, "notification-server")
	if err != nil {
		log.Printf("Failed to get service: %v", err)
		return
	}

	fmt.Printf("Service address: %s\n", addr)
}

// Example_getServiceList 演示如何获取服务的所有实例
func Example_getServiceList() {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"localhost:2379"},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	sd := discovery.NewServiceDiscovery(client)

	// 获取服务的所有实例地址
	ctx := context.Background()
	addresses, err := sd.GetServiceList(ctx, "notification-server")
	if err != nil {
		log.Printf("Failed to get service list: %v", err)
		return
	}

	fmt.Printf("Found %d instances:\n", len(addresses))
	for i, addr := range addresses {
		fmt.Printf("  Instance %d: %s\n", i+1, addr)
	}
}

// Example_watchService 演示如何监听服务变化
func Example_watchService() {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"localhost:2379"},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	sd := discovery.NewServiceDiscovery(client)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 监听服务变化
	fmt.Println("Watching service changes...")
	sd.WatchService(ctx, "notification-server", func(eventType discovery.EventType, addr string) {
		switch eventType {
		case discovery.EventTypeAdd:
			fmt.Printf("Service added: %s\n", addr)
		case discovery.EventTypeDelete:
			fmt.Printf("Service deleted: %s\n", addr)
		}
	})
}

// Example_getAllServices 演示如何获取所有注册的服务
func Example_getAllServices() {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"localhost:2379"},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	sd := discovery.NewServiceDiscovery(client)

	ctx := context.Background()
	services, err := sd.GetAllServices(ctx)
	if err != nil {
		log.Printf("Failed to get all services: %v", err)
		return
	}

	fmt.Printf("Found %d services:\n", len(services))
	for name, addrs := range services {
		fmt.Printf("  %s: %v\n", name, addrs)
	}
}

// Example_dialService 演示如何创建到服务的 gRPC 连接
func Example_dialService() {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"localhost:2379"},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	sd := discovery.NewServiceDiscovery(client)

	ctx := context.Background()
	conn, err := sd.DialService(ctx, "notification-server")
	if err != nil {
		log.Printf("Failed to dial service: %v", err)
		return
	}
	defer conn.Close()

	fmt.Println("Successfully connected to service")

	// 使用连接创建 gRPC 客户端
	// client := notificationpb.NewNotificationServiceClient(conn)
	// ...
}

// Example_waitForService 演示如何等待服务上线
func Example_waitForService() {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"localhost:2379"},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	sd := discovery.NewServiceDiscovery(client)

	ctx := context.Background()
	fmt.Println("Waiting for service to be online...")
	addr, err := sd.WaitForService(ctx, "notification-server", 10*time.Second)
	if err != nil {
		log.Printf("Service not available: %v", err)
		return
	}

	fmt.Printf("Service is online at: %s\n", addr)
}

// Example_startWatch 演示如何使用缓存模式
func Example_startWatch() {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"localhost:2379"},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	sd := discovery.NewServiceDiscovery(client)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 启动后台监听，自动更新缓存
	sd.StartWatch(ctx)

	// 等待一下让缓存初始化
	time.Sleep(100 * time.Millisecond)

	// 从缓存中快速获取服务地址（不需要访问 etcd）
	addr, err := sd.GetCachedService("notification-server")
	if err != nil {
		log.Printf("Failed to get cached service: %v", err)
		return
	}

	fmt.Printf("Cached service address: %s\n", addr)

	// 获取服务的所有实例
	addrs, err := sd.GetCachedServiceList("notification-server")
	if err != nil {
		log.Printf("Failed to get cached service list: %v", err)
		return
	}

	fmt.Printf("Found %d cached instances\n", len(addrs))
}

// Example_completeWorkflow 演示完整的使用流程
func Example_completeWorkflow() {
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

	ctx := context.Background()

	// 3. 等待服务上线（最多等待 10 秒）
	fmt.Println("Waiting for service...")
	addr, err := sd.WaitForService(ctx, "notification-server", 10*time.Second)
	if err != nil {
		log.Printf("Service not available: %v", err)
		return
	}
	fmt.Printf("Service found at: %s\n", addr)

	// 4. 创建 gRPC 连接
	conn, err := sd.DialService(ctx, "notification-server")
	if err != nil {
		log.Printf("Failed to connect: %v", err)
		return
	}
	defer conn.Close()

	fmt.Println("Connected successfully")

	// 5. 创建 gRPC 客户端并调用服务
	// client := notificationpb.NewNotificationServiceClient(conn)
	// resp, err := client.Send(ctx, &notificationpb.SendRequest{...})
	// ...

	// 6. 在后台监听服务变化
	go sd.WatchService(ctx, "notification-server", func(eventType discovery.EventType, addr string) {
		fmt.Printf("Service %s: %s\n", eventType, addr)
		// 处理服务上线/下线事件
		// 例如：重新建立连接、更新连接池等
	})

	// 继续执行其他业务逻辑...
}
