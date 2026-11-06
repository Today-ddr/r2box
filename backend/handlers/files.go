package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"r2box/httpx"
	"r2box/models"
	"r2box/services"
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

// FileListItemWithURL 文件列表项（包含下载入口）
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
		httpx.WriteJSONError(w, http.StatusMethodNotAllowed, "方法不允许")
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

	files, total, err := models.ListFiles(h.db, page, limit)
	if err != nil {
		httpx.WriteJSONError(w, http.StatusInternalServerError, "获取文件列表失败")
		return
	}

	// 下载统一走后端 302，这里只给出本站入口，不再签发 R2 直链
	filesWithURL := make([]FileListItemWithURL, len(files))
	for i, file := range files {
		filesWithURL[i] = FileListItemWithURL{FileListItem: file}
		if file.UploadStatus == "completed" && time.Now().Before(file.ExpiresAt) {
			filesWithURL[i].DownloadURL = DownloadPath(file.ID)
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

// GetDownloadURL 校验后临时签发并跳转
func (h *FilesHandler) GetDownloadURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpx.WriteJSONError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	// /api/files/:id/download
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		httpx.WriteJSONError(w, http.StatusBadRequest, "无效的请求")
		return
	}
	fileID := parts[3]

	file, err := models.GetFileByID(h.db, fileID)
	if err != nil {
		httpx.WriteJSONError(w, http.StatusNotFound, "文件不存在")
		return
	}

	if file.UploadStatus != "completed" {
		httpx.WriteJSONError(w, http.StatusGone, "文件不可用")
		return
	}

	if time.Now().After(file.ExpiresAt) {
		httpx.WriteJSONError(w, http.StatusGone, "文件已过期")
		return
	}

	// 只签 5 分钟：外链语义由本站地址保证，R2 直链越短越安全
	downloadURL, err := h.r2Service.GenerateDownloadURL(file.R2Key, file.Filename, 5*time.Minute)
	if err != nil {
		httpx.WriteJSONError(w, http.StatusInternalServerError, "生成下载链接失败")
		return
	}

	http.Redirect(w, r, downloadURL, http.StatusFound)
}

// Delete 删除文件
func (h *FilesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		httpx.WriteJSONError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	// /api/files/:id
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		httpx.WriteJSONError(w, http.StatusBadRequest, "无效的请求")
		return
	}
	fileID := parts[3]

	file, err := models.GetFileByID(h.db, fileID)
	if err != nil {
		httpx.WriteJSONError(w, http.StatusNotFound, "文件不存在")
		return
	}

	// 对象可能已经被过期清理删掉了，这里的删除应该是幂等的
	if err := h.r2Service.DeleteObject(file.R2Key); err != nil {
		httpx.WriteJSONError(w, http.StatusInternalServerError, "删除文件失败")
		return
	}

	if err := models.DeleteFile(h.db, fileID); err != nil {
		httpx.WriteJSONError(w, http.StatusInternalServerError, "删除文件记录失败")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "文件已删除",
	})
}
