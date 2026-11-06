package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"r2box/httpx"
	"r2box/models"
	"r2box/services"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// UploadHandler 上传处理器
type UploadHandler struct {
	db           *sql.DB
	r2Service    *services.R2Service
	maxFileSize  int64
	totalStorage int64
}

// NewUploadHandler 创建上传处理器
func NewUploadHandler(db *sql.DB, r2Service *services.R2Service, maxFileSize, totalStorage int64) *UploadHandler {
	return &UploadHandler{
		db:           db,
		r2Service:    r2Service,
		maxFileSize:  maxFileSize,
		totalStorage: totalStorage,
	}
}

// admit 上传准入：体积与配额。
// R2 不支持在预签名 URL 上约束体积，这里只能基于客户端声明做前置判断，
// 真实大小在确认上传时用 HeadObject 复核。
func (h *UploadHandler) admit(size int64) (int, string) {
	if size <= 0 {
		return http.StatusBadRequest, "文件大小无效"
	}
	if size > h.maxFileSize {
		return http.StatusRequestEntityTooLarge, "文件大小超过限制"
	}

	committed, err := models.GetCommittedStorage(h.db)
	if err != nil {
		log.Printf("[Upload] 读取已用配额失败: %v", err)
		return http.StatusInternalServerError, "读取存储用量失败"
	}

	if committed+size > h.totalStorage {
		return http.StatusInsufficientStorage, "存储空间不足，请清理文件后重试"
	}

	return 0, ""
}

// verifyObject 确认对象真的落在 R2 上，并用真实大小覆盖声明值
func (h *UploadHandler) verifyObject(file *models.File) (int, string) {
	info, err := h.r2Service.HeadObject(file.R2Key)
	if err != nil {
		if services.IsNotFound(err) {
			file.UpdateStatus(h.db, "failed")
			return http.StatusBadRequest, "对象未上传成功"
		}
		log.Printf("[Upload] 读取对象信息失败: %v", err)
		return http.StatusBadGateway, "存储服务暂时不可用，请重试"
	}

	committed, err := models.GetCommittedStorage(h.db)
	if err != nil {
		log.Printf("[Upload] 读取已用配额失败: %v", err)
		return http.StatusInternalServerError, "读取存储用量失败"
	}

	// committed 里包含本条记录声明的 size，先扣掉再加真实值
	if info.Size > h.maxFileSize || committed-file.Size+info.Size > h.totalStorage {
		if delErr := h.r2Service.DeleteObject(file.R2Key); delErr != nil {
			log.Printf("[Upload] 删除超限对象失败: %v", delErr)
		}
		file.UpdateStatus(h.db, "failed")

		if info.Size > h.maxFileSize {
			return http.StatusRequestEntityTooLarge, "文件大小超过限制，已拒绝"
		}
		return http.StatusInsufficientStorage, "存储空间不足，已拒绝"
	}

	contentType := info.ContentType
	if contentType == "" {
		contentType = file.ContentType
	}

	if err := file.SetActualSize(h.db, info.Size, contentType); err != nil {
		log.Printf("[Upload] 更新文件大小失败: %v", err)
	}

	return 0, ""
}

// expiredStatus 判断记录是否还能被确认
func expiredStatus(status string) bool {
	return status == "deleted" || status == "cancelled"
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

// DownloadPath 对外的下载入口，统一经过后端做过期校验后再跳转
func DownloadPath(fileID string) string {
	return "/api/files/" + fileID + "/download"
}

// normalizeExpiresIn 校验过期天数（支持30秒测试：-30表示30秒）
func normalizeExpiresIn(v int) int {
	switch v {
	case -30, 1, 3, 7, 30:
		return v
	default:
		return 7
	}
}

// GeneratePresignURL 生成预签名上传 URL（小文件）
func (h *UploadHandler) GeneratePresignURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpx.WriteJSONError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	var req PresignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteJSONError(w, http.StatusBadRequest, "无效的请求")
		return
	}

	if status, msg := h.admit(req.Size); status != 0 {
		httpx.WriteJSONError(w, status, msg)
		return
	}

	req.ExpiresIn = normalizeExpiresIn(req.ExpiresIn)

	// 创建文件记录
	file := &models.File{
		Filename:     req.Filename,
		Size:         req.Size,
		ContentType:  req.ContentType,
		ExpiresIn:    req.ExpiresIn,
		UploadStatus: "pending",
	}

	if err := file.Create(h.db); err != nil {
		log.Printf("[Upload] 创建文件记录失败: %v", err)
		httpx.WriteJSONError(w, http.StatusInternalServerError, "创建文件记录失败")
		return
	}

	// 生成预签名上传 URL
	uploadURL, err := h.r2Service.GenerateUploadURL(file.R2Key, req.ContentType, time.Hour)
	if err != nil {
		httpx.WriteJSONError(w, http.StatusInternalServerError, "生成上传 URL 失败")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(PresignResponse{
		FileID:      file.ID,
		UploadURL:   uploadURL,
		DownloadURL: DownloadPath(file.ID),
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
		httpx.WriteJSONError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	var req ConfirmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteJSONError(w, http.StatusBadRequest, "无效的请求")
		return
	}

	file, err := models.GetFileByID(h.db, req.FileID)
	if err != nil {
		httpx.WriteJSONError(w, http.StatusNotFound, "文件不存在")
		return
	}

	if expiredStatus(file.UploadStatus) {
		httpx.WriteJSONError(w, http.StatusGone, "该上传已失效，请重新上传")
		return
	}

	if status, msg := h.verifyObject(file); status != 0 {
		httpx.WriteJSONError(w, status, msg)
		return
	}

	if err := file.UpdateStatus(h.db, "completed"); err != nil {
		log.Printf("[Upload] 更新状态失败: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ConfirmResponse{
		Success:     true,
		Message:     "上传确认成功",
		DownloadURL: DownloadPath(file.ID),
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

// calcPartSize 在 20MiB 和「不超过 9000 个分片」之间取较大者
func calcPartSize(size int64) int64 {
	const (
		defaultPartSize = 20 << 20
		maxParts        = 9000
	)
	partSize := int64(defaultPartSize)
	if needed := (size + maxParts - 1) / maxParts; needed > partSize {
		partSize = needed
	}
	return partSize
}

// InitiateMultipartUpload 初始化分片上传
func (h *UploadHandler) InitiateMultipartUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpx.WriteJSONError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	var req MultipartInitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteJSONError(w, http.StatusBadRequest, "无效的请求")
		return
	}

	if status, msg := h.admit(req.Size); status != 0 {
		httpx.WriteJSONError(w, status, msg)
		return
	}

	req.ExpiresIn = normalizeExpiresIn(req.ExpiresIn)

	file := &models.File{
		Filename:     req.Filename,
		Size:         req.Size,
		ContentType:  req.ContentType,
		ExpiresIn:    req.ExpiresIn,
		UploadStatus: "pending",
	}

	if err := file.Create(h.db); err != nil {
		log.Printf("[Upload] 创建文件记录失败: %v", err)
		httpx.WriteJSONError(w, http.StatusInternalServerError, "创建文件记录失败")
		return
	}

	uploadID, err := h.r2Service.InitiateMultipartUpload(file.R2Key, req.ContentType)
	if err != nil {
		file.UpdateStatus(h.db, "failed")
		httpx.WriteJSONError(w, http.StatusInternalServerError, "初始化分片上传失败")
		return
	}

	// uploadID 必须落库，否则前端一刷新就再也无法 complete 或 abort
	if err := file.SetUploadID(h.db, uploadID); err != nil {
		log.Printf("[Upload] 保存 uploadID 失败: %v", err)
	}

	partSize := calcPartSize(req.Size)
	totalParts := int((req.Size + partSize - 1) / partSize)

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

// resolveUploadID 以数据库中的 uploadID 为准，请求体与之冲突时拒绝
func resolveUploadID(file *models.File, requested string) (string, bool) {
	if file.UploadID == "" {
		return requested, requested != ""
	}
	if requested == "" || requested == file.UploadID {
		return file.UploadID, true
	}
	return "", false
}

// GenerateMultipartPresignURL 生成分片预签名 URL
func (h *UploadHandler) GenerateMultipartPresignURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpx.WriteJSONError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	var req MultipartPresignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteJSONError(w, http.StatusBadRequest, "无效的请求")
		return
	}

	file, err := models.GetFileByID(h.db, req.FileID)
	if err != nil {
		httpx.WriteJSONError(w, http.StatusNotFound, "文件不存在")
		return
	}

	uploadID, ok := resolveUploadID(file, req.UploadID)
	if !ok {
		httpx.WriteJSONError(w, http.StatusBadRequest, "upload_id 不匹配")
		return
	}

	uploadURL, err := h.r2Service.GenerateMultipartUploadURL(file.R2Key, uploadID, req.PartNumber)
	if err != nil {
		httpx.WriteJSONError(w, http.StatusInternalServerError, "生成分片上传 URL 失败")
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

// CancelUploadRequest 取消上传请求
type CancelUploadRequest struct {
	FileID   string `json:"file_id"`
	UploadID string `json:"upload_id,omitempty"` // 分片上传时需要
}

// CancelUploadResponse 取消上传响应
type CancelUploadResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// CompleteMultipartUpload 完成分片上传
func (h *UploadHandler) CompleteMultipartUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpx.WriteJSONError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	var req MultipartCompleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[Upload] 解析请求失败: %v", err)
		httpx.WriteJSONError(w, http.StatusBadRequest, "无效的请求")
		return
	}

	file, err := models.GetFileByID(h.db, req.FileID)
	if err != nil {
		httpx.WriteJSONError(w, http.StatusNotFound, "文件不存在")
		return
	}

	if expiredStatus(file.UploadStatus) {
		httpx.WriteJSONError(w, http.StatusGone, "该上传已失效，请重新上传")
		return
	}

	uploadID, ok := resolveUploadID(file, req.UploadID)
	if !ok {
		httpx.WriteJSONError(w, http.StatusBadRequest, "upload_id 不匹配")
		return
	}

	// 以 R2 中实际存在的分片为准，避免客户端漏传或乱序
	r2Parts, err := h.r2Service.ListParts(file.R2Key, uploadID)
	if err != nil {
		log.Printf("[Upload] 列出分片失败: %v", err)
		httpx.WriteJSONError(w, http.StatusInternalServerError, "列出分片失败")
		return
	}

	var completeParts []types.CompletedPart
	for _, p := range r2Parts {
		partNum := *p.PartNumber
		etag := *p.ETag
		completeParts = append(completeParts, types.CompletedPart{
			PartNumber: &partNum,
			ETag:       &etag,
		})
	}

	if err := h.r2Service.CompleteMultipartUpload(file.R2Key, uploadID, completeParts); err != nil {
		log.Printf("[Upload] 完成分片上传失败: %v", err)
		httpx.WriteJSONError(w, http.StatusInternalServerError, "完成分片上传失败")
		return
	}

	if status, msg := h.verifyObject(file); status != 0 {
		httpx.WriteJSONError(w, status, msg)
		return
	}

	if err := file.UpdateStatus(h.db, "completed"); err != nil {
		log.Printf("[Upload] 更新状态失败: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(MultipartCompleteResponse{
		FileID:      file.ID,
		DownloadURL: DownloadPath(file.ID),
		ShortURL:    "/s/" + file.ShortCode,
		ExpiresAt:   file.ExpiresAt.Format(time.RFC3339),
	})
}

// CancelUpload 取消上传
func (h *UploadHandler) CancelUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpx.WriteJSONError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	var req CancelUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteJSONError(w, http.StatusBadRequest, "无效的请求")
		return
	}

	if req.FileID == "" {
		httpx.WriteJSONError(w, http.StatusBadRequest, "缺少 file_id")
		return
	}

	log.Printf("[Upload] 取消上传: file_id=%s", req.FileID)

	file, err := models.GetFileByID(h.db, req.FileID)
	if err != nil {
		// 文件不存在也返回成功，因为目标是清理
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(CancelUploadResponse{
			Success: true,
			Message: "文件记录不存在，无需清理",
		})
		return
	}

	uploadID, ok := resolveUploadID(file, req.UploadID)
	if !ok {
		httpx.WriteJSONError(w, http.StatusBadRequest, "upload_id 不匹配")
		return
	}

	// 如果是分片上传，终止分片上传
	if uploadID != "" {
		if err := h.r2Service.AbortMultipartUpload(file.R2Key, uploadID); err != nil {
			log.Printf("[Upload] 终止分片上传失败: %v", err)
			// 继续执行，尝试删除可能已存在的对象
		}
	}

	// 尝试删除 R2 中可能已存在的对象（小文件上传或部分完成的上传）
	if err := h.r2Service.DeleteObject(file.R2Key); err != nil {
		log.Printf("[Upload] 删除 R2 对象失败: %v", err)
	}

	// 更新文件状态为已取消
	file.UpdateStatus(h.db, "cancelled")

	// 删除数据库记录
	if _, err := h.db.Exec("DELETE FROM files WHERE id = ?", req.FileID); err != nil {
		log.Printf("[Upload] 删除文件记录失败: %v", err)
	}

	log.Printf("[Upload] 上传已取消: file_id=%s", req.FileID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(CancelUploadResponse{
		Success: true,
		Message: "上传已取消，R2 数据已清理",
	})
}
