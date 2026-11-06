package models

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"math/big"
	"time"

	"github.com/google/uuid"

	"r2box/database"
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
	UploadID     string    `json:"-"`
}

// FileListItem 文件列表项（包含剩余时间）
type FileListItem struct {
	File
	RemainingTime string `json:"remaining_time"`
}

const fileColumns = `id, filename, r2_key, size, content_type, expires_in,
	created_at, expires_at, upload_status, COALESCE(short_code, ''), COALESCE(upload_id, ''),
	created_at_unix, expires_at_unix`

// scanFile 从查询结果读取一条文件记录
func scanFile(row interface{ Scan(...interface{}) error }) (*File, error) {
	var f File
	var createdAt, expiresAt sql.NullString
	var createdAtUnix, expiresAtUnix sql.NullInt64

	err := row.Scan(&f.ID, &f.Filename, &f.R2Key, &f.Size, &f.ContentType, &f.ExpiresIn,
		&createdAt, &expiresAt, &f.UploadStatus, &f.ShortCode, &f.UploadID,
		&createdAtUnix, &expiresAtUnix)
	if err != nil {
		return nil, err
	}

	// 优先解析文本时间，失败时退回整数列，保证展示与筛选口径一致
	if t, ok := database.ParseSQLiteTime(createdAt.String); ok {
		f.CreatedAt = t
	} else if createdAtUnix.Valid {
		f.CreatedAt = time.Unix(createdAtUnix.Int64, 0)
	}

	if t, ok := database.ParseSQLiteTime(expiresAt.String); ok {
		f.ExpiresAt = t
	} else if expiresAtUnix.Valid {
		f.ExpiresAt = time.Unix(expiresAtUnix.Int64, 0)
	}

	return &f, nil
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
			INSERT INTO files (id, filename, r2_key, size, content_type, expires_in,
				created_at, expires_at, upload_status, short_code, created_at_unix, expires_at_unix)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, f.ID, f.Filename, f.R2Key, f.Size, f.ContentType, f.ExpiresIn,
			f.CreatedAt, f.ExpiresAt, f.UploadStatus, f.ShortCode,
			f.CreatedAt.Unix(), f.ExpiresAt.Unix())

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
	if err == nil {
		f.UploadStatus = status
	}
	return err
}

// SetUploadID 保存分片上传的 uploadID
func (f *File) SetUploadID(db *sql.DB, uploadID string) error {
	_, err := db.Exec("UPDATE files SET upload_id = ?, upload_status = 'uploading' WHERE id = ?", uploadID, f.ID)
	if err == nil {
		f.UploadID = uploadID
		f.UploadStatus = "uploading"
	}
	return err
}

// SetActualSize 用对象存储中的真实大小覆盖声明值
func (f *File) SetActualSize(db *sql.DB, size int64, contentType string) error {
	_, err := db.Exec("UPDATE files SET size = ?, content_type = ? WHERE id = ?", size, contentType, f.ID)
	if err == nil {
		f.Size = size
		f.ContentType = contentType
	}
	return err
}

// GetFileByID 根据 ID 获取文件
func GetFileByID(db *sql.DB, id string) (*File, error) {
	f, err := scanFile(db.QueryRow(`SELECT `+fileColumns+` FROM files WHERE id = ?`, id))
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("文件不存在")
	}
	return f, err
}

// GetFileByShortCode 根据短码获取文件
func GetFileByShortCode(db *sql.DB, shortCode string) (*File, error) {
	f, err := scanFile(db.QueryRow(`SELECT `+fileColumns+` FROM files WHERE short_code = ?`, shortCode))
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
		SELECT `+fileColumns+`
		FROM files
		WHERE upload_status IN ('completed', 'deleted')
		ORDER BY created_at_unix DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var files []FileListItem
	for rows.Next() {
		f, err := scanFile(rows)
		if err != nil {
			return nil, 0, err
		}

		files = append(files, FileListItem{
			File:          *f,
			RemainingTime: formatDuration(time.Until(f.ExpiresAt)),
		})
	}

	return files, total, rows.Err()
}

// DeleteFile 删除文件记录
func DeleteFile(db *sql.DB, id string) error {
	_, err := db.Exec("DELETE FROM files WHERE id = ?", id)
	return err
}

// GetExpiredFiles 获取已过期且未删除的文件
func GetExpiredFiles(db *sql.DB) ([]File, error) {
	rows, err := db.Query(`
		SELECT `+fileColumns+`
		FROM files
		WHERE expires_at_unix < ? AND upload_status = 'completed'
	`, time.Now().Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []File
	for rows.Next() {
		f, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		files = append(files, *f)
	}
	return files, rows.Err()
}

// GetStaleUploads 获取超时未完成的上传记录
func GetStaleUploads(db *sql.DB, before time.Time) ([]File, error) {
	rows, err := db.Query(`
		SELECT `+fileColumns+`
		FROM files
		WHERE upload_status IN ('pending', 'uploading') AND created_at_unix < ?
	`, before.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []File
	for rows.Next() {
		f, err := scanFile(rows)
		if err != nil {
			return nil, err
		}
		files = append(files, *f)
	}
	return files, rows.Err()
}

// DeleteStaleFailed 删除陈旧的失败记录
func DeleteStaleFailed(db *sql.DB, before time.Time) (int64, error) {
	res, err := db.Exec(`
		DELETE FROM files WHERE upload_status = 'failed' AND created_at_unix < ?
	`, before.Unix())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// GetCommittedStorage 已占用额度：在途 + 已完成
func GetCommittedStorage(db *sql.DB) (int64, error) {
	var committed int64
	err := db.QueryRow(`
		SELECT COALESCE(SUM(size), 0) FROM files
		WHERE upload_status IN ('pending', 'uploading', 'completed')
	`).Scan(&committed)
	return committed, err
}

// GetStorageStats 获取存储统计
func GetStorageStats(db *sql.DB, totalStorage int64) (map[string]interface{}, error) {
	var usedSpace int64
	var inFlight int64
	var fileCount int

	err := db.QueryRow(`
		SELECT COALESCE(SUM(CASE WHEN upload_status = 'completed' THEN size ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN upload_status IN ('pending', 'uploading') THEN size ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN upload_status = 'completed' THEN 1 ELSE 0 END), 0)
		FROM files WHERE upload_status IN ('pending', 'uploading', 'completed')
	`).Scan(&usedSpace, &inFlight, &fileCount)
	if err != nil {
		return nil, err
	}

	committedSpace := usedSpace + inFlight
	now := time.Now()
	nowUnix := now.Unix()

	// 本地时区的次日零点，不能用 Truncate(24h)（那是按 UTC 零点截断的）
	y, m, d := now.Date()
	endOfToday := time.Date(y, m, d+1, 0, 0, 0, 0, now.Location()).Unix()

	// 今天过期的文件数
	var expiringToday int
	db.QueryRow(`
		SELECT COUNT(*) FROM files
		WHERE upload_status = 'completed' AND expires_at_unix >= ? AND expires_at_unix < ?
	`, nowUnix, endOfToday).Scan(&expiringToday)

	// 本周过期的文件数
	var expiringThisWeek int
	db.QueryRow(`
		SELECT COUNT(*) FROM files
		WHERE upload_status = 'completed' AND expires_at_unix >= ? AND expires_at_unix < ?
	`, nowUnix, now.Add(7*24*time.Hour).Unix()).Scan(&expiringThisWeek)

	usagePercent := 0.0
	if totalStorage > 0 {
		usagePercent = float64(committedSpace) / float64(totalStorage) * 100
	}

	return map[string]interface{}{
		"usedSpace":           usedSpace,
		"inFlightSpace":       inFlight,
		"committedSpace":      committedSpace,
		"totalSpace":          totalStorage,
		"usedSpaceFormatted":  formatBytes(usedSpace),
		"inFlightFormatted":   formatBytes(inFlight),
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
