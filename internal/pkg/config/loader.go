package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// ConfigLoader 配置加载器接口
type ConfigLoader interface {
	// Load 加载配置到指定的结构体
	Load(key string, target interface{}) error

	// GetString 获取字符串配置
	GetString(key string) string

	// GetInt 获取整数配置
	GetInt(key string) int

	// GetBool 获取布尔配置
	GetBool(key string) bool

	// GetDuration 获取时间间隔配置
	GetDuration(key string) time.Duration
}

// ViperConfigLoader 基于 Viper 的配置加载器
type ViperConfigLoader struct {
	v *viper.Viper
}

// NewViperConfigLoader 创建 Viper 配置加载器
func NewViperConfigLoader() *ViperConfigLoader {
	return &ViperConfigLoader{
		v: viper.GetViper(),
	}
}

// NewViperConfigLoaderWithViper 使用指定的 Viper 实例创建加载器
func NewViperConfigLoaderWithViper(v *viper.Viper) *ViperConfigLoader {
	return &ViperConfigLoader{v: v}
}

// Load 加载配置到指定的结构体
func (l *ViperConfigLoader) Load(key string, target interface{}) error {
	err := l.v.UnmarshalKey(key, target, viper.DecodeHook(viper.DecoderConfigOption(TagName("yaml"))))
	if err != nil {
		return fmt.Errorf("failed to unmarshal config key %s: %w", key, err)
	}
	return nil
}

// GetString 获取字符串配置
func (l *ViperConfigLoader) GetString(key string) string {
	return l.v.GetString(key)
}

// GetInt 获取整数配置
func (l *ViperConfigLoader) GetInt(key string) int {
	return l.v.GetInt(key)
}

// GetBool 获取布尔配置
func (l *ViperConfigLoader) GetBool(key string) bool {
	return l.v.GetBool(key)
}

// GetDuration 获取时间间隔配置
func (l *ViperConfigLoader) GetDuration(key string) time.Duration {
	return l.v.GetDuration(key)
}

// InitViperConfig 初始化 Viper 配置（辅助函数）
func InitViperConfig(configPaths ...string) error {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// 添加配置文件搜索路径
	if len(configPaths) == 0 {
		configPaths = []string{
			"./config/platform",
			"../../config/platform",
			".",
		}
	}

	for _, path := range configPaths {
		viper.AddConfigPath(path)
	}

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	return nil
}

// 确保 ViperConfigLoader 实现了 ConfigLoader 接口
var _ ConfigLoader = (*ViperConfigLoader)(nil)
