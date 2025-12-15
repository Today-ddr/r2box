package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"r2box/models"
	"r2box/services"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// UploadHandler 上传处理器
type UploadHandler struct {
	db          *sql.DB
	r2Service   *services.R2Service
	maxFileSize int64
}

// NewUploadHandler 创建上传处理器
func NewUploadHandler(db *sql.DB, r2Service *services.R2Service, maxFileSize int64) *UploadHandler {
	return &UploadHandler{
		db:          db,
		r2Service:   r2Service,
		maxFileSize: maxFileSize,
	}
}

// PresignRequest 预签名请求
type PresignRequest struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	ExpiresIn   int    `json:"expires_in"` // 过期天数: 1, 3, 7, 30
}

// PresignResponse 预签名响应
type PresignResponse struct {
	FileID      string `json:"file_id"`
	UploadURL   string `json:"upload_url"`
	DownloadURL string `json:"download_url"`
	ShortURL    string `json:"short_url"`
	ExpiresAt   string `json:"expires_at"`
}

// GeneratePresignURL 生成预签名上传 URL（小文件）
func (h *UploadHandler) GeneratePresignURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"方法不允许"}`, http.StatusMethodNotAllowed)
		return
	}

	var req PresignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"无效的请求"}`, http.StatusBadRequest)
		return
	}

	// 验证文件大小
	if req.Size > h.maxFileSize {
		http.Error(w, `{"error":"文件大小超过限制"}`, http.StatusBadRequest)
		return
	}

	// 验证过期时间（支持30秒测试：-30表示30秒）
	if req.ExpiresIn != -30 && req.ExpiresIn != 1 && req.ExpiresIn != 3 && req.ExpiresIn != 7 && req.ExpiresIn != 30 {
		req.ExpiresIn = 7 // 默认 7 天
	}

	// 创建文件记录
	file := &models.File{
		Filename:     req.Filename,
		Size:         req.Size,
		ContentType:  req.ContentType,
		ExpiresIn:    req.ExpiresIn,
		UploadStatus: "pending",
	}

	if err := file.Create(h.db); err != nil {
		http.Error(w, `{"error":"创建文件记录失败"}`, http.StatusInternalServerError)
		return
	}

	// 生成预签名上传 URL
	uploadURL, err := h.r2Service.GenerateUploadURL(file.R2Key, req.ContentType, time.Hour)
	if err != nil {
		http.Error(w, `{"error":"生成上传 URL 失败"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(PresignResponse{
		FileID:      file.ID,
		UploadURL:   uploadURL,
		DownloadURL: "/api/files/" + file.ID + "/download",
		ShortURL:    "/s/" + file.ShortCode,
		ExpiresAt:   file.ExpiresAt.Format(time.RFC3339),
	})
}

// ConfirmRequest 确认上传完成请求
type ConfirmRequest struct {
	FileID string `json:"file_id"`
}

// ConfirmResponse 确认上传完成响应
type ConfirmResponse struct {
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	DownloadURL string `json:"download_url"`
	ShortURL    string `json:"short_url"`
}

// ConfirmUpload 确认上传完成（小文件）
func (h *UploadHandler) ConfirmUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"方法不允许"}`, http.StatusMethodNotAllowed)
		return
	}

	var req ConfirmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"无效的请求"}`, http.StatusBadRequest)
		return
	}

	// 获取文件记录
	file, err := models.GetFileByID(h.db, req.FileID)
	if err != nil {
		http.Error(w, `{"error":"文件不存在"}`, http.StatusNotFound)
		return
	}

	// 更新文件状态
	file.UpdateStatus(h.db, "completed")

	// 生成 R2 预签名下载直链（有效期与文件过期时间一致）
	downloadURL, err := h.r2Service.GenerateDownloadURL(file.R2Key, file.Filename, time.Until(file.ExpiresAt))
	if err != nil {
		log.Printf("[Upload] 生成下载 URL 失败: %v", err)
		// 即使生成失败也返回成功，使用备用链接
		downloadURL = "/api/files/" + file.ID + "/download"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ConfirmResponse{
		Success:     true,
		Message:     "上传确认成功",
		DownloadURL: downloadURL,
		ShortURL:    "/s/" + file.ShortCode,
	})
}

// MultipartInitRequest 分片上传初始化请求
type MultipartInitRequest struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	ExpiresIn   int    `json:"expires_in"`
}

// MultipartInitResponse 分片上传初始化响应
type MultipartInitResponse struct {
	FileID     string `json:"file_id"`
	UploadID   string `json:"upload_id"`
	PartSize   int64  `json:"part_size"`
	TotalParts int    `json:"total_parts"`
}

// InitiateMultipartUpload 初始化分片上传
func (h *UploadHandler) InitiateMultipartUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"方法不允许"}`, http.StatusMethodNotAllowed)
		return
	}

	var req MultipartInitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"无效的请求"}`, http.StatusBadRequest)
		return
	}

	// 验证文件大小
	if req.Size > h.maxFileSize {
		http.Error(w, `{"error":"文件大小超过限制"}`, http.StatusBadRequest)
		return
	}

	// 验证过期时间（支持30秒测试：-30表示30秒）
	if req.ExpiresIn != -30 && req.ExpiresIn != 1 && req.ExpiresIn != 3 && req.ExpiresIn != 7 && req.ExpiresIn != 30 {
		req.ExpiresIn = 7
	}

	// 创建文件记录
	file := &models.File{
		Filename:     req.Filename,
		Size:         req.Size,
		ContentType:  req.ContentType,
		ExpiresIn:    req.ExpiresIn,
		UploadStatus: "pending",
	}

	if err := file.Create(h.db); err != nil {
		http.Error(w, `{"error":"创建文件记录失败"}`, http.StatusInternalServerError)
		return
	}

	// 初始化分片上传
	uploadID, err := h.r2Service.InitiateMultipartUpload(file.R2Key, req.ContentType)
	if err != nil {
		http.Error(w, `{"error":"初始化分片上传失败"}`, http.StatusInternalServerError)
		return
	}

	// 计算分片信息
	partSize := int64(20 * 1024 * 1024) // 20MB
	totalParts := int((req.Size + partSize - 1) / partSize)

	// 保存 uploadID 到数据库
	h.db.Exec("UPDATE files SET upload_status = 'uploading' WHERE id = ?", file.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(MultipartInitResponse{
		FileID:     file.ID,
		UploadID:   uploadID,
		PartSize:   partSize,
		TotalParts: totalParts,
	})
}

// MultipartPresignRequest 分片预签名请求
type MultipartPresignRequest struct {
	FileID     string `json:"file_id"`
	UploadID   string `json:"upload_id"`
	PartNumber int32  `json:"part_number"`
}

// MultipartPresignResponse 分片预签名响应
type MultipartPresignResponse struct {
	UploadURL  string `json:"upload_url"`
	PartNumber int32  `json:"part_number"`
}

// GenerateMultipartPresignURL 生成分片预签名 URL
func (h *UploadHandler) GenerateMultipartPresignURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"方法不允许"}`, http.StatusMethodNotAllowed)
		return
	}

	var req MultipartPresignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"无效的请求"}`, http.StatusBadRequest)
		return
	}

	// 获取文件记录
	file, err := models.GetFileByID(h.db, req.FileID)
	if err != nil {
		http.Error(w, `{"error":"文件不存在"}`, http.StatusNotFound)
		return
	}

	// 生成分片预签名 URL
	uploadURL, err := h.r2Service.GenerateMultipartUploadURL(file.R2Key, req.UploadID, req.PartNumber)
	if err != nil {
		http.Error(w, `{"error":"生成分片上传 URL 失败"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(MultipartPresignResponse{
		UploadURL:  uploadURL,
		PartNumber: req.PartNumber,
	})
}

// MultipartCompleteRequest 完成分片上传请求
type MultipartCompleteRequest struct {
	FileID   string `json:"file_id"`
	UploadID string `json:"upload_id"`
	Parts    []struct {
		PartNumber int32  `json:"part_number"`
		ETag       string `json:"etag"`
	} `json:"parts"`
}

// MultipartCompleteResponse 完成分片上传响应
type MultipartCompleteResponse struct {
	FileID      string `json:"file_id"`
	DownloadURL string `json:"download_url"`
	ShortURL    string `json:"short_url"`
	ExpiresAt   string `json:"expires_at"`
}

// CompleteMultipartUpload 完成分片上传
func (h *UploadHandler) CompleteMultipartUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"方法不允许"}`, http.StatusMethodNotAllowed)
		return
	}

	var req MultipartCompleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[Upload] 解析请求失败: %v", err)
		http.Error(w, `{"error":"无效的请求"}`, http.StatusBadRequest)
		return
	}


	// 获取文件记录
	file, err := models.GetFileByID(h.db, req.FileID)
	if err != nil {
		http.Error(w, `{"error":"文件不存在"}`, http.StatusNotFound)
		return
	}

	// 获取 R2 中实际存在的分片并完成上传
	r2Parts, err := h.r2Service.ListParts(file.R2Key, req.UploadID)
	if err != nil {
		log.Printf("[Upload] 列出分片失败: %v", err)
		http.Error(w, `{"error":"列出分片失败"}`, http.StatusInternalServerError)
		return
	}

	// 使用 R2 返回的分片信息来完成上传
	var completeParts []types.CompletedPart
	for _, p := range r2Parts {
		partNum := *p.PartNumber
		etag := *p.ETag
		completeParts = append(completeParts, types.CompletedPart{
			PartNumber: &partNum,
			ETag:       &etag,
		})
	}

	// 完成分片上传
	if err := h.r2Service.CompleteMultipartUpload(file.R2Key, req.UploadID, completeParts); err != nil {
		log.Printf("[Upload] 完成分片上传失败: %v", err)
		http.Error(w, `{"error":"完成分片上传失败"}`, http.StatusInternalServerError)
		return
	}

	// 更新文件状态
	file.UpdateStatus(h.db, "completed")

	// 生成 R2 预签名下载直链（有效期与文件过期时间一致）
	downloadURL, err := h.r2Service.GenerateDownloadURL(file.R2Key, file.Filename, time.Until(file.ExpiresAt))
	if err != nil {
		log.Printf("[Upload] 生成下载 URL 失败: %v", err)
		// 即使生成失败也返回成功，使用备用链接
		downloadURL = "/api/files/" + file.ID + "/download"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(MultipartCompleteResponse{
		FileID:      file.ID,
		DownloadURL: downloadURL,
		ShortURL:    "/s/" + file.ShortCode,
		ExpiresAt:   file.ExpiresAt.Format(time.RFC3339),
	})
}
