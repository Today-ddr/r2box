package handlers

import (
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"r2box/database"
)

func setupQuotaDB(t *testing.T) {
	t.Helper()
	if err := database.Init(filepath.Join(t.TempDir(), "quota.db")); err != nil {
		t.Fatalf("初始化测试库失败: %v", err)
	}
	t.Cleanup(func() { database.Close() })
}

func seedFile(t *testing.T, id, status string, size int64) {
	t.Helper()
	now := time.Now().Unix()
	if _, err := database.DB.Exec(`
		INSERT INTO files (id, filename, r2_key, size, content_type, expires_in,
			created_at, expires_at, upload_status, short_code, created_at_unix, expires_at_unix)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, "f.txt", "r2box/"+id, size, "text/plain", 7,
		time.Unix(now, 0), time.Unix(now+86400, 0), status, "c"+id, now, now+86400); err != nil {
		t.Fatalf("插入测试数据失败: %v", err)
	}
}

func TestAdmit(t *testing.T) {
	setupQuotaDB(t)
	seedFile(t, "used", "completed", 600)

	h := &UploadHandler{
		db:           database.DB,
		maxFileSize:  1000,
		totalStorage: 1000,
	}

	cases := []struct {
		name   string
		size   int64
		status int
	}{
		{"零字节", 0, http.StatusBadRequest},
		{"负数", -1, http.StatusBadRequest},
		{"超过单文件上限", 1001, http.StatusRequestEntityTooLarge},
		{"配额不足", 401, http.StatusInsufficientStorage},
		{"刚好占满", 400, 0},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			status, msg := h.admit(c.size)
			if status != c.status {
				t.Errorf("admit(%d) = (%d, %q), want status %d", c.size, status, msg, c.status)
			}
		})
	}
}

func TestAdmitIgnoresFailedRecords(t *testing.T) {
	setupQuotaDB(t)
	seedFile(t, "failed", "failed", 900)
	seedFile(t, "cancelled", "cancelled", 900)

	h := &UploadHandler{
		db:           database.DB,
		maxFileSize:  1000,
		totalStorage: 1000,
	}

	if status, msg := h.admit(500); status != 0 {
		t.Errorf("失败/取消的记录不应占用配额，got (%d, %q)", status, msg)
	}
}
