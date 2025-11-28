package main

import (
	"log"

	"github.com/serendipityConfusion/notification-platform/cmd/platform/ioc"
	"github.com/serendipityConfusion/notification-platform/internal/pkg/config"
)

func main() {
	// 1. 初始化配置
	if err := initConfig(); err != nil {
		log.Fatalf("[Main] Failed to initialize config: %v", err)
	}
	log.Println("[Main] Configuration loaded successfully")

	// 2. 通过 wire 初始化应用（依赖注入）
	app := ioc.InitGrpcServer()
	log.Println("[Main] Application initialized successfully")

	// 3. 运行应用
	if err := app.Run(); err != nil {
		log.Fatalf("[Main] Application error: %v", err)
	}

	log.Println("[Main] Application exited successfully")
}

// initConfig 初始化配置
func initConfig() error {
	// 使用配置加载器的辅助函数初始化 Viper
	return config.InitViperConfig(
		"./config/platform",     // 生产环境路径
		"../../config/platform", // 开发/测试环境路径
		".",                     // 当前目录
	)
}
