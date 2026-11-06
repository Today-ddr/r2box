package database

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// 老版本的建表语句，用于验证迁移能直接跑在存量库上
const legacySchema = `
CREATE TABLE system_config (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE files (
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
CREATE TABLE rate_limits (
	ip TEXT PRIMARY KEY,
	request_count INTEGER DEFAULT 0,
	window_start DATETIME DEFAULT CURRENT_TIMESTAMP,
	blocked_until DATETIME,
	failed_attempts INTEGER DEFAULT 0
);
INSERT INTO files (id, filename, r2_key, size, content_type, expires_in, created_at, expires_at, upload_status)
VALUES ('f1', 'a.txt', 'r2box/f1.txt', 100, 'text/plain', 7,
	'2026-11-05 10:00:00+08:00', '2026-11-12 10:00:00+08:00', 'completed');
`

func TestInitMigratesLegacyDatabase(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy.db")

	raw, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(legacySchema); err != nil {
		t.Fatal(err)
	}
	raw.Close()

	if err := Init(dbPath); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	defer Close()

	var createdAtUnix, expiresAtUnix sql.NullInt64
	err = DB.QueryRow("SELECT created_at_unix, expires_at_unix FROM files WHERE id = 'f1'").
		Scan(&createdAtUnix, &expiresAtUnix)
	if err != nil {
		t.Fatal(err)
	}
	if !createdAtUnix.Valid || !expiresAtUnix.Valid {
		t.Fatal("文本时间未被回填成 Unix 秒")
	}
	// 2026-11-05 10:00:00+08:00 == 2026-11-05 02:00:00 UTC
	want := int64(1793844000)
	if createdAtUnix.Int64 != want {
		t.Errorf("created_at_unix = %d, want %d", createdAtUnix.Int64, want)
	}
	if diff := expiresAtUnix.Int64 - createdAtUnix.Int64; diff != 7*24*3600 {
		t.Errorf("过期间隔 = %d 秒, want %d", diff, 7*24*3600)
	}

	// 限流表应已被重建为整数列
	var count int
	if err := DB.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('rate_limits') WHERE name = 'window_start_unix'
	`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Error("rate_limits 未迁移到 window_start_unix")
	}
}
