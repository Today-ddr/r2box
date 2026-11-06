package config

import (
	"os"
	"strconv"
	"strings"
)

// Config 应用配置
type Config struct {
	// 服务器配置
	Port string

	// 受信代理网段（CIDR），只有来自这些网段的请求才解析 X-Forwarded-For
	TrustedProxies []string

	// 文件限制
	MaxFileSize  int64
	TotalStorage int64

	// 数据库路径
	DatabasePath string
}

// Load 从环境变量加载配置
func Load() *Config {
	return &Config{
		Port:           getEnv("PORT", "8080"),
		TrustedProxies: getEnvList("TRUSTED_PROXIES"),
		MaxFileSize:    getEnvInt64("MAX_FILE_SIZE", 5*1024*1024*1024),  // 默认 5GB
		TotalStorage:   getEnvInt64("TOTAL_STORAGE", 10*1024*1024*1024), // 默认 10GB
		DatabasePath:   getEnv("DATABASE_PATH", "./data/r2box.db"),
	}
}

// getEnv 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvList 获取逗号分隔的环境变量
func getEnvList(key string) []string {
	value := os.Getenv(key)
	if value == "" {
		return nil
	}

	var items []string
	for _, item := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
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
