package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

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

	// WAL 让读写并发不再互相阻塞，busy_timeout 处理写冲突时的重试
	dsn := dbPath
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	dsn += sep + "_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL"

	// 打开数据库连接
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return fmt.Errorf("打开数据库失败: %w", err)
	}

	// SQLite 的写入是串行的，限制连接数避免无谓的锁竞争
	db.SetMaxOpenConns(4)

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

	-- 文件元数据表
	CREATE TABLE IF NOT EXISTS files (
		id TEXT PRIMARY KEY,
		filename TEXT NOT NULL,
		r2_key TEXT NOT NULL UNIQUE,
		size INTEGER NOT NULL,
		content_type TEXT NOT NULL,
		expires_in INTEGER NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		expires_at DATETIME NOT NULL,
		upload_status TEXT DEFAULT 'pending',
		short_code TEXT,
		upload_id TEXT,
		created_at_unix INTEGER,
		expires_at_unix INTEGER
	);

	-- 创建索引
	CREATE INDEX IF NOT EXISTS idx_files_expires_at ON files(expires_at);
	CREATE INDEX IF NOT EXISTS idx_files_created_at ON files(created_at);

	-- 速率限制表
	CREATE TABLE IF NOT EXISTS rate_limits (
		ip TEXT PRIMARY KEY,
		request_count INTEGER DEFAULT 0,
		window_start_unix INTEGER NOT NULL DEFAULT 0,
		blocked_until_unix INTEGER,
		failed_attempts INTEGER DEFAULT 0
	);
	`

	_, err := DB.Exec(schema)
	if err != nil {
		return err
	}

	if err := migrateRateLimits(); err != nil {
		return err
	}

	// 迁移：老库补列。列已存在时 ALTER 会报错，忽略即可。
	migrations := []string{
		"ALTER TABLE files ADD COLUMN short_code TEXT",
		"ALTER TABLE files ADD COLUMN upload_id TEXT",
		"ALTER TABLE files ADD COLUMN created_at_unix INTEGER",
		"ALTER TABLE files ADD COLUMN expires_at_unix INTEGER",
	}
	for _, stmt := range migrations {
		if _, err := DB.Exec(stmt); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return fmt.Errorf("迁移失败(%s): %w", stmt, err)
		}
	}

	if _, err := DB.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_files_short_code ON files(short_code)"); err != nil {
		return err
	}
	if _, err := DB.Exec("CREATE INDEX IF NOT EXISTS idx_files_expires_unix ON files(expires_at_unix)"); err != nil {
		return err
	}
	if _, err := DB.Exec("CREATE INDEX IF NOT EXISTS idx_files_status_unix ON files(upload_status, created_at_unix)"); err != nil {
		return err
	}

	return backfillUnixTimes()
}

// migrateRateLimits 把旧版按 DATETIME 存储的限流表换成 Unix 整数列。
// 限流数据是短生命周期的临时状态，重建表不会丢失业务数据。
func migrateRateLimits() error {
	var count int
	err := DB.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('rate_limits') WHERE name = 'window_start_unix'
	`).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	if _, err := DB.Exec("DROP TABLE IF EXISTS rate_limits"); err != nil {
		return err
	}

	_, err = DB.Exec(`
		CREATE TABLE rate_limits (
			ip TEXT PRIMARY KEY,
			request_count INTEGER DEFAULT 0,
			window_start_unix INTEGER NOT NULL DEFAULT 0,
			blocked_until_unix INTEGER,
			failed_attempts INTEGER DEFAULT 0
		)
	`)
	return err
}

// sqliteTimeLayouts 覆盖驱动写入（带时区偏移）与 CURRENT_TIMESTAMP（UTC，无偏移）两种格式
var sqliteTimeLayouts = []string{
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02T15:04:05.999999999-07:00",
	time.RFC3339Nano,
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05",
}

// ParseSQLiteTime 解析 SQLite 中的时间文本
func ParseSQLiteTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range sqliteTimeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// backfillUnixTimes 把历史记录的文本时间转换成 Unix 秒，幂等
func backfillUnixTimes() error {
	rows, err := DB.Query(`
		SELECT id, created_at, expires_at, expires_in
		FROM files WHERE created_at_unix IS NULL OR expires_at_unix IS NULL
	`)
	if err != nil {
		return err
	}

	type pending struct {
		id                 string
		createdAt, expires string
		expiresIn          int
	}
	var items []pending
	for rows.Next() {
		var p pending
		var createdAt, expiresAt sql.NullString
		if err := rows.Scan(&p.id, &createdAt, &expiresAt, &p.expiresIn); err != nil {
			rows.Close()
			return err
		}
		p.createdAt, p.expires = createdAt.String, expiresAt.String
		items = append(items, p)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	if len(items) == 0 {
		return nil
	}

	failed := 0
	for _, p := range items {
		createdAt, ok := ParseSQLiteTime(p.createdAt)
		if !ok {
			createdAt = time.Now()
			failed++
		}

		expiresAt, ok := ParseSQLiteTime(p.expires)
		if !ok {
			expiresAt = createdAt.Add(time.Duration(p.expiresIn) * 24 * time.Hour)
			failed++
		}

		if _, err := DB.Exec(`
			UPDATE files SET created_at_unix = ?, expires_at_unix = ? WHERE id = ?
		`, createdAt.Unix(), expiresAt.Unix(), p.id); err != nil {
			return err
		}
	}

	if failed > 0 {
		log.Printf("[DB] 时间迁移：%d 行中的时间无法解析，已按过期天数推算", failed)
	}
	log.Printf("[DB] 时间迁移完成，处理 %d 行", len(items))

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

// IsPasswordSet 检查密码是否已设置
func IsPasswordSet() bool {
	pwd, _ := GetConfig("password_hash")
	return pwd != ""
}

// GetPasswordHash 获取密码哈希
func GetPasswordHash() (string, error) {
	return GetConfig("password_hash")
}

// SetPasswordHash 设置密码哈希
func SetPasswordHash(hash string) error {
	return SetConfig("password_hash", hash)
}
