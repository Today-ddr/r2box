package config

import (
	"os"
	"strconv"
)

// Config 应用配置
type Config struct {
	// 服务器配置
	Port string

	// 认证配置
	AccessToken string

	// 文件限制
	MaxFileSize   int64
	TotalStorage  int64

	// 数据库路径
	DatabasePath string
}

// Load 从环境变量加载配置
func Load() *Config {
	return &Config{
		Port:         getEnv("PORT", "8080"),
		AccessToken:  getEnv("ACCESS_TOKEN", ""),
		MaxFileSize:  getEnvInt64("MAX_FILE_SIZE", 5*1024*1024*1024), // 默认 5GB
		TotalStorage: getEnvInt64("TOTAL_STORAGE", 10*1024*1024*1024), // 默认 10GB
		DatabasePath: getEnv("DATABASE_PATH", "./data/r2box.db"),
	}
}

// getEnv 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt64 获取 int64 类型的环境变量
func getEnvInt64(key string, defaultValue int64) int64 {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intValue
		}
	}
	return defaultValue
}
