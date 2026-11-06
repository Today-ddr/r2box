package models

import (
	"path/filepath"
	"testing"
	"time"

	"r2box/database"
)

func setupTestDB(t *testing.T) {
	t.Helper()
	if err := database.Init(filepath.Join(t.TempDir(), "test.db")); err != nil {
		t.Fatalf("初始化测试库失败: %v", err)
	}
	t.Cleanup(func() { database.Close() })
}

func insertFile(t *testing.T, id, status string, size int64, expiresIn int) {
	t.Helper()

	created := time.Now().Unix()
	expires := created + int64(expiresIn)*86400

	if _, err := database.DB.Exec(`
		INSERT INTO files (id, filename, r2_key, size, content_type, expires_in,
			created_at, expires_at, upload_status, short_code, created_at_unix, expires_at_unix)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, "f.txt", "r2box/"+id, size, "text/plain", expiresIn,
		time.Unix(created, 0), time.Unix(expires, 0), status, "code"+id, created, expires); err != nil {
		t.Fatalf("插入测试数据失败: %v", err)
	}
}

func TestGetCommittedStorage(t *testing.T) {
	setupTestDB(t)
	insertFile(t, "a", "pending", 100, 7)
	insertFile(t, "b", "uploading", 200, 7)
	insertFile(t, "c", "completed", 300, 7)
	insertFile(t, "d", "failed", 400, 7)
	insertFile(t, "e", "cancelled", 500, 7)

	got, err := GetCommittedStorage(database.DB)
	if err != nil {
		t.Fatal(err)
	}
	if got != 600 {
		t.Errorf("GetCommittedStorage = %d, want 600", got)
	}
}

func TestGetExpiredFiles(t *testing.T) {
	setupTestDB(t)

	insertFile(t, "expired", "completed", 100, -1)
	insertFile(t, "alive", "completed", 100, 7)
	insertFile(t, "pending", "pending", 100, -1)

	files, err := GetExpiredFiles(database.DB)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].ID != "expired" {
		t.Errorf("GetExpiredFiles = %+v, want 只包含 expired", files)
	}
}

func TestGetStaleUploads(t *testing.T) {
	setupTestDB(t)

	insertFile(t, "stale", "uploading", 100, 7)
	insertFile(t, "fresh", "uploading", 100, 7)

	if _, err := database.DB.Exec(
		"UPDATE files SET created_at_unix = created_at_unix - 25*3600 WHERE id = 'stale'",
	); err != nil {
		t.Fatal(err)
	}

	files, err := GetStaleUploads(database.DB, time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].ID != "stale" {
		t.Errorf("GetStaleUploads = %+v, want 只包含 stale", files)
	}
}

func TestGetStorageStats(t *testing.T) {
	setupTestDB(t)
	insertFile(t, "done", "completed", 100, 7)
	insertFile(t, "inflight", "pending", 50, 7)

	stats, err := GetStorageStats(database.DB, 1000)
	if err != nil {
		t.Fatal(err)
	}

	if stats["usedSpace"].(int64) != 100 {
		t.Errorf("usedSpace = %v, want 100", stats["usedSpace"])
	}
	if stats["inFlightSpace"].(int64) != 50 {
		t.Errorf("inFlightSpace = %v, want 50", stats["inFlightSpace"])
	}
	if stats["committedSpace"].(int64) != 150 {
		t.Errorf("committedSpace = %v, want 150", stats["committedSpace"])
	}
	if pct := stats["usagePercent"].(float64); pct != 15 {
		t.Errorf("usagePercent = %v, want 15", pct)
	}
}
