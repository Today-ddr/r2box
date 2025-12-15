package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// DB 数据库实例
var DB *sql.DB

// Init 初始化数据库
func Init(dbPath string) error {
	// 确保数据目录存在
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}

	// 打开数据库连接
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("打开数据库失败: %w", err)
	}

	// 测试连接
	if err := db.Ping(); err != nil {
		return fmt.Errorf("数据库连接测试失败: %w", err)
	}

	DB = db

	// 创建表结构
	if err := createTables(); err != nil {
		return fmt.Errorf("创建表结构失败: %w", err)
	}

	return nil
}

// createTables 创建数据库表
func createTables() error {
	schema := `
	-- 系统配置表
	CREATE TABLE IF NOT EXISTS system_config (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- 文件元数据表（基础结构）
	CREATE TABLE IF NOT EXISTS files (
		id TEXT PRIMARY KEY,
		filename TEXT NOT NULL,
		r2_key TEXT NOT NULL UNIQUE,
		size INTEGER NOT NULL,
		content_type TEXT NOT NULL,
		expires_in INTEGER NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		expires_at DATETIME NOT NULL,
		upload_status TEXT DEFAULT 'pending'
	);

	-- 创建索引
	CREATE INDEX IF NOT EXISTS idx_files_expires_at ON files(expires_at);
	CREATE INDEX IF NOT EXISTS idx_files_created_at ON files(created_at);

	-- 速率限制表
	CREATE TABLE IF NOT EXISTS rate_limits (
		ip TEXT PRIMARY KEY,
		request_count INTEGER DEFAULT 0,
		window_start DATETIME DEFAULT CURRENT_TIMESTAMP,
		blocked_until DATETIME,
		failed_attempts INTEGER DEFAULT 0
	);
	`

	_, err := DB.Exec(schema)
	if err != nil {
		return err
	}

	// 迁移：为 files 表添加 short_code 字段（如果不存在）
	DB.Exec("ALTER TABLE files ADD COLUMN short_code TEXT")
	DB.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_files_short_code ON files(short_code)")

	return nil
}

// Close 关闭数据库连接
func Close() error {
	if DB != nil {
		return DB.Close()
	}
	return nil
}

// GetConfig 获取配置项
func GetConfig(key string) (string, error) {
	var value string
	err := DB.QueryRow("SELECT value FROM system_config WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// SetConfig 设置配置项
func SetConfig(key, value string) error {
	_, err := DB.Exec(`
		INSERT INTO system_config (key, value, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value = ?, updated_at = CURRENT_TIMESTAMP
	`, key, value, value)
	return err
}

// IsR2Configured 检查 R2 是否已配置
func IsR2Configured() (bool, error) {
	configured, err := GetConfig("r2_configured")
	if err != nil {
		return false, err
	}
	return configured == "true", nil
}
