package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"r2box/models"
	"r2box/services"
	"strconv"
	"strings"
	"time"
)

// FilesHandler 文件管理处理器
type FilesHandler struct {
	db        *sql.DB
	r2Service *services.R2Service
}

// NewFilesHandler 创建文件管理处理器
func NewFilesHandler(db *sql.DB, r2Service *services.R2Service) *FilesHandler {
	return &FilesHandler{
		db:        db,
		r2Service: r2Service,
	}
}

// FileListItemWithURL 文件列表项（包含 R2 直链）
type FileListItemWithURL struct {
	models.FileListItem
	DownloadURL string `json:"download_url"`
}

// ListResponse 文件列表响应
type ListResponse struct {
	Files []FileListItemWithURL `json:"files"`
	Total int                   `json:"total"`
	Page  int                   `json:"page"`
	Limit int                   `json:"limit"`
}

// List 获取文件列表
func (h *FilesHandler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"方法不允许"}`, http.StatusMethodNotAllowed)
		return
	}

	// 解析分页参数
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 20
	}

	// 获取文件列表
	files, total, err := models.ListFiles(h.db, page, limit)
	if err != nil {
		http.Error(w, `{"error":"获取文件列表失败"}`, http.StatusInternalServerError)
		return
	}

	// 为每个文件生成 R2 预签名直链
	filesWithURL := make([]FileListItemWithURL, len(files))
	for i, file := range files {
		filesWithURL[i] = FileListItemWithURL{
			FileListItem: file,
		}
		// 只为未过期且已完成的文件生成直链
		if file.UploadStatus == "completed" && time.Now().Before(file.ExpiresAt) {
			downloadURL, err := h.r2Service.GenerateDownloadURL(file.R2Key, file.Filename, time.Until(file.ExpiresAt))
			if err == nil {
				filesWithURL[i].DownloadURL = downloadURL
			} else {
				// 生成失败时使用备用链接
				filesWithURL[i].DownloadURL = "/api/files/" + file.ID + "/download"
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ListResponse{
		Files: filesWithURL,
		Total: total,
		Page:  page,
		Limit: limit,
	})
}

// GetDownloadURL 获取下载 URL
func (h *FilesHandler) GetDownloadURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"方法不允许"}`, http.StatusMethodNotAllowed)
		return
	}

	// 从 URL 路径中提取文件 ID
	// /api/files/:id/download
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		http.Error(w, `{"error":"无效的请求"}`, http.StatusBadRequest)
		return
	}
	fileID := parts[3]

	// 获取文件记录
	file, err := models.GetFileByID(h.db, fileID)
	if err != nil {
		http.Error(w, `{"error":"文件不存在"}`, http.StatusNotFound)
		return
	}

	// 检查文件是否已过期
	if time.Now().After(file.ExpiresAt) {
		http.Error(w, `{"error":"文件已过期"}`, http.StatusGone)
		return
	}

	// 生成下载预签名 URL（使用原始文件名）
	downloadURL, err := h.r2Service.GenerateDownloadURL(file.R2Key, file.Filename, 24*time.Hour)
	if err != nil {
		http.Error(w, `{"error":"生成下载 URL 失败"}`, http.StatusInternalServerError)
		return
	}

	// 重定向到预签名 URL
	http.Redirect(w, r, downloadURL, http.StatusFound)
}

// Delete 删除文件
func (h *FilesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, `{"error":"方法不允许"}`, http.StatusMethodNotAllowed)
		return
	}

	// 从 URL 路径中提取文件 ID
	// /api/files/:id
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		http.Error(w, `{"error":"无效的请求"}`, http.StatusBadRequest)
		return
	}
	fileID := parts[3]

	// 获取文件记录
	file, err := models.GetFileByID(h.db, fileID)
	if err != nil {
		http.Error(w, `{"error":"文件不存在"}`, http.StatusNotFound)
		return
	}

	// 从 R2 删除对象
	if err := h.r2Service.DeleteObject(file.R2Key); err != nil {
		http.Error(w, `{"error":"删除文件失败"}`, http.StatusInternalServerError)
		return
	}

	// 从数据库删除记录
	if err := models.DeleteFile(h.db, fileID); err != nil {
		http.Error(w, `{"error":"删除文件记录失败"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "文件已删除",
	})
}
