package ioc

import (
	"github.com/serendipityConfusion/notification-platform/internal/pkg/config"
)

// InitConfigLoader 初始化配置加载器
// 使用全局的 viper 实例创建配置加载器
func InitConfigLoader() *config.ViperConfigLoader {
	return config.NewViperConfigLoader()
}
