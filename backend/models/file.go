package models

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"
)

// File 文件元数据
type File struct {
	ID           string    `json:"id"`
	Filename     string    `json:"filename"`
	R2Key        string    `json:"r2_key"`
	Size         int64     `json:"size"`
	ContentType  string    `json:"content_type"`
	ExpiresIn    int       `json:"expires_in"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	UploadStatus string    `json:"upload_status"`
	ShortCode    string    `json:"short_code"`
}

// FileListItem 文件列表项（包含剩余时间）
type FileListItem struct {
	File
	RemainingTime string `json:"remaining_time"`
}

// generateShortCode 生成6位短码
func generateShortCode() (string, error) {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, 6)
	for i := range result {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", err
		}
		result[i] = chars[n.Int64()]
	}
	return string(result), nil
}

// Create 创建文件记录
func (f *File) Create(db *sql.DB) error {
	f.ID = uuid.New().String()
	f.CreatedAt = time.Now()
	// 支持30秒测试（-30表示30秒）
	if f.ExpiresIn == -30 {
		f.ExpiresAt = f.CreatedAt.Add(30 * time.Second)
	} else {
		f.ExpiresAt = f.CreatedAt.Add(time.Duration(f.ExpiresIn) * 24 * time.Hour)
	}

	// 使用 UUID 作为 R2 key，避免文件名中的特殊字符导致问题
	// 获取文件扩展名
	ext := ""
	for i := len(f.Filename) - 1; i >= 0; i-- {
		if f.Filename[i] == '.' {
			ext = f.Filename[i:]
			break
		}
	}
	// 格式: r2box/UUID.扩展名
	f.R2Key = fmt.Sprintf("r2box/%s%s", f.ID, ext)

	// 生成短码，重试直到成功
	for i := 0; i < 10; i++ {
		code, err := generateShortCode()
		if err != nil {
			return err
		}
		f.ShortCode = code

		_, err = db.Exec(`
			INSERT INTO files (id, filename, r2_key, size, content_type, expires_in, created_at, expires_at, upload_status, short_code)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, f.ID, f.Filename, f.R2Key, f.Size, f.ContentType, f.ExpiresIn, f.CreatedAt, f.ExpiresAt, f.UploadStatus, f.ShortCode)

		if err == nil {
			return nil
		}
		// 如果是唯一性冲突，重试
		if err.Error() != "UNIQUE constraint failed: files.short_code" {
			return err
		}
	}

	return fmt.Errorf("生成短码失败")
}

// UpdateStatus 更新上传状态
func (f *File) UpdateStatus(db *sql.DB, status string) error {
	_, err := db.Exec("UPDATE files SET upload_status = ? WHERE id = ?", status, f.ID)
	return err
}

// GetByID 根据 ID 获取文件
func GetFileByID(db *sql.DB, id string) (*File, error) {
	f := &File{}
	err := db.QueryRow(`
		SELECT id, filename, r2_key, size, content_type, expires_in, created_at, expires_at, upload_status, COALESCE(short_code, '')
		FROM files WHERE id = ?
	`, id).Scan(&f.ID, &f.Filename, &f.R2Key, &f.Size, &f.ContentType, &f.ExpiresIn, &f.CreatedAt, &f.ExpiresAt, &f.UploadStatus, &f.ShortCode)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("文件不存在")
	}
	return f, err
}

// GetFileByShortCode 根据短码获取文件
func GetFileByShortCode(db *sql.DB, shortCode string) (*File, error) {
	f := &File{}
	err := db.QueryRow(`
		SELECT id, filename, r2_key, size, content_type, expires_in, created_at, expires_at, upload_status, COALESCE(short_code, '')
		FROM files WHERE short_code = ?
	`, shortCode).Scan(&f.ID, &f.Filename, &f.R2Key, &f.Size, &f.ContentType, &f.ExpiresIn, &f.CreatedAt, &f.ExpiresAt, &f.UploadStatus, &f.ShortCode)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("文件不存在")
	}
	return f, err
}

// ListFiles 获取文件列表
func ListFiles(db *sql.DB, page, limit int) ([]FileListItem, int, error) {
	offset := (page - 1) * limit

	// 获取总数（包含已完成和已删除的文件）
	var total int
	err := db.QueryRow("SELECT COUNT(*) FROM files WHERE upload_status IN ('completed', 'deleted')").Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// 获取文件列表（包含已完成和已删除的文件）
	rows, err := db.Query(`
		SELECT id, filename, r2_key, size, content_type, expires_in, created_at, expires_at, upload_status, COALESCE(short_code, '')
		FROM files
		WHERE upload_status IN ('completed', 'deleted')
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var files []FileListItem
	for rows.Next() {
		var f File
		err := rows.Scan(&f.ID, &f.Filename, &f.R2Key, &f.Size, &f.ContentType, &f.ExpiresIn, &f.CreatedAt, &f.ExpiresAt, &f.UploadStatus, &f.ShortCode)
		if err != nil {
			return nil, 0, err
		}

		// 计算剩余时间
		remaining := time.Until(f.ExpiresAt)
		remainingStr := formatDuration(remaining)

		files = append(files, FileListItem{
			File:          f,
			RemainingTime: remainingStr,
		})
	}

	return files, total, nil
}

// DeleteFile 删除文件记录
func DeleteFile(db *sql.DB, id string) error {
	_, err := db.Exec("DELETE FROM files WHERE id = ?", id)
	return err
}

// GetExpiredFiles 获取已过期且未删除的文件
func GetExpiredFiles(db *sql.DB) ([]File, error) {
	now := time.Now().Format("2006-01-02 15:04:05")
	rows, err := db.Query(`
		SELECT id, filename, r2_key, size, content_type, expires_in, created_at, expires_at, upload_status, COALESCE(short_code, '')
		FROM files
		WHERE expires_at < ? AND upload_status = 'completed'
	`, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []File
	for rows.Next() {
		var f File
		err := rows.Scan(&f.ID, &f.Filename, &f.R2Key, &f.Size, &f.ContentType, &f.ExpiresIn, &f.CreatedAt, &f.ExpiresAt, &f.UploadStatus, &f.ShortCode)
		if err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, nil
}

// GetStorageStats 获取存储统计
func GetStorageStats(db *sql.DB, totalStorage int64) (map[string]interface{}, error) {
	var usedSpace int64
	var fileCount int

	err := db.QueryRow("SELECT COALESCE(SUM(size), 0), COUNT(*) FROM files WHERE upload_status = 'completed'").Scan(&usedSpace, &fileCount)
	if err != nil {
		return nil, err
	}

	// 今天过期的文件数
	var expiringToday int
	today := time.Now().Truncate(24 * time.Hour).Add(24 * time.Hour)
	db.QueryRow("SELECT COUNT(*) FROM files WHERE expires_at < ? AND upload_status = 'completed'", today).Scan(&expiringToday)

	// 本周过期的文件数
	var expiringThisWeek int
	nextWeek := time.Now().Add(7 * 24 * time.Hour)
	db.QueryRow("SELECT COUNT(*) FROM files WHERE expires_at < ? AND upload_status = 'completed'", nextWeek).Scan(&expiringThisWeek)

	usagePercent := float64(usedSpace) / float64(totalStorage) * 100

	return map[string]interface{}{
		"usedSpace":           usedSpace,
		"totalSpace":          totalStorage,
		"usedSpaceFormatted":  formatBytes(usedSpace),
		"totalSpaceFormatted": formatBytes(totalStorage),
		"usagePercent":        usagePercent,
		"fileCount":           fileCount,
		"expiringToday":       expiringToday,
		"expiringThisWeek":    expiringThisWeek,
	}, nil
}

// formatDuration 格式化时间间隔
func formatDuration(d time.Duration) string {
	if d < 0 {
		return "已过期"
	}

	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%d天 %d小时 %d分钟", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%d小时 %d分钟", hours, minutes)
	} else {
		return fmt.Sprintf("%d分钟", minutes)
	}
}

// formatBytes 格式化字节数
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
