package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"r2box/services"
)

// SetupHandler R2 配置向导处理器
type SetupHandler struct {
	db              *sql.DB
	onConfigChanged func() // 配置变更回调
}

// NewSetupHandler 创建配置向导处理器
func NewSetupHandler(db *sql.DB, onConfigChanged func()) *SetupHandler {
	return &SetupHandler{
		db:              db,
		onConfigChanged: onConfigChanged,
	}
}

// StatusResponse 配置状态响应
type StatusResponse struct {
	Configured bool                   `json:"configured"`
	Config     map[string]interface{} `json:"config,omitempty"`
}

// Status 获取 R2 配置状态
func (h *SetupHandler) Status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"方法不允许"}`, http.StatusMethodNotAllowed)
		return
	}

	log.Println("[Setup] 获取 R2 配置状态")

	var r2Configured string
	err := h.db.QueryRow("SELECT value FROM system_config WHERE key = 'r2_configured'").Scan(&r2Configured)

	configured := err == nil && r2Configured == "true"

	response := StatusResponse{
		Configured: configured,
	}

	// 如果已配置，返回配置信息（隐藏敏感信息）
	if configured {
		var endpoint, bucketName string
		h.db.QueryRow("SELECT value FROM system_config WHERE key = 'r2_endpoint'").Scan(&endpoint)
		h.db.QueryRow("SELECT value FROM system_config WHERE key = 'r2_bucket_name'").Scan(&bucketName)

		response.Config = map[string]interface{}{
			"endpoint":    endpoint,
			"bucket_name": bucketName,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ConfigRequest 配置请求
type ConfigRequest struct {
	Endpoint        string `json:"endpoint"` // 完整端点 URL，如 https://xxx.r2.cloudflarestorage.com
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	BucketName      string `json:"bucket_name"`
}

// ConfigResponse 配置响应
type ConfigResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// SaveConfig 保存 R2 配置
func (h *SetupHandler) SaveConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"方法不允许"}`, http.StatusMethodNotAllowed)
		return
	}

	var req ConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[Setup] 解析请求失败: %v", err)
		http.Error(w, `{"error":"无效的请求"}`, http.StatusBadRequest)
		return
	}

	log.Printf("[Setup] 保存 R2 配置: endpoint=%s, bucket=%s", req.Endpoint, req.BucketName)

	// 验证必填字段
	if req.Endpoint == "" || req.AccessKeyID == "" || req.SecretAccessKey == "" || req.BucketName == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ConfigResponse{
			Success: false,
			Message: "所有字段都是必填的",
		})
		return
	}

	// 保存配置到数据库
	tx, err := h.db.Begin()
	if err != nil {
		log.Printf("[Setup] 开始事务失败: %v", err)
		http.Error(w, `{"error":"数据库错误"}`, http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	configs := map[string]string{
		"r2_endpoint":          req.Endpoint,
		"r2_access_key_id":     req.AccessKeyID,
		"r2_secret_access_key": req.SecretAccessKey,
		"r2_bucket_name":       req.BucketName,
		"r2_configured":        "true",
	}

	for key, value := range configs {
		_, err := tx.Exec(`
			INSERT INTO system_config (key, value, updated_at)
			VALUES (?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(key) DO UPDATE SET value = ?, updated_at = CURRENT_TIMESTAMP
		`, key, value, value)
		if err != nil {
			log.Printf("[Setup] 保存配置项 %s 失败: %v", key, err)
			http.Error(w, `{"error":"保存配置失败"}`, http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("[Setup] 提交事务失败: %v", err)
		http.Error(w, `{"error":"提交配置失败"}`, http.StatusInternalServerError)
		return
	}

	log.Println("[Setup] R2 配置保存成功")

	// 触发配置变更回调
	if h.onConfigChanged != nil {
		log.Println("[Setup] 触发配置变更回调，重新加载 R2 服务")
		h.onConfigChanged()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ConfigResponse{
		Success: true,
		Message: "配置保存成功",
	})
}

// TestRequest 测试连接请求
type TestRequest struct {
	Endpoint        string `json:"endpoint"`
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	BucketName      string `json:"bucket_name"`
}

// TestResponse 测试连接响应
type TestResponse struct {
	Success    bool                   `json:"success"`
	Message    string                 `json:"message"`
	BucketInfo map[string]interface{} `json:"bucket_info,omitempty"`
}

// TestConnection 测试 R2 连接
func (h *SetupHandler) TestConnection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"方法不允许"}`, http.StatusMethodNotAllowed)
		return
	}

	var req TestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[Setup] 解析测试请求失败: %v", err)
		http.Error(w, `{"error":"无效的请求"}`, http.StatusBadRequest)
		return
	}

	log.Printf("[Setup] 测试 R2 连接: endpoint=%s, bucket=%s", req.Endpoint, req.BucketName)

	// 创建临时 R2 服务实例
	r2Config := &services.R2Config{
		Endpoint:        req.Endpoint,
		AccessKeyID:     req.AccessKeyID,
		SecretAccessKey: req.SecretAccessKey,
		BucketName:      req.BucketName,
	}

	r2Service, err := services.NewR2ServiceWithConfig(r2Config)
	if err != nil {
		log.Printf("[Setup] 创建 R2 客户端失败: %v", err)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TestResponse{
			Success: false,
			Message: "创建 R2 客户端失败: " + err.Error(),
		})
		return
	}

	// 测试连接
	if err := r2Service.TestConnection(); err != nil {
		log.Printf("[Setup] 连接测试失败: %v", err)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TestResponse{
			Success: false,
			Message: "连接测试失败: " + err.Error(),
		})
		return
	}

	log.Println("[Setup] 连接测试成功")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(TestResponse{
		Success: true,
		Message: "连接测试成功，存储桶可用",
		BucketInfo: map[string]interface{}{
			"name": req.BucketName,
		},
	})
}
