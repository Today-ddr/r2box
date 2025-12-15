package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"r2box/middleware"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	accessToken string
	db          *sql.DB
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(accessToken string, db *sql.DB) *AuthHandler {
	return &AuthHandler{
		accessToken: accessToken,
		db:          db,
	}
}

// LoginRequest 登录请求
type LoginRequest struct {
	Token string `json:"token"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Success   bool   `json:"success"`
	NeedSetup bool   `json:"need_setup"`
	Message   string `json:"message,omitempty"`
}

// Login 登录验证
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"方法不允许"}`, http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"无效的请求"}`, http.StatusBadRequest)
		return
	}

	// 验证 token
	if req.Token != h.accessToken {
		// 记录失败尝试
		ip := getClientIP(r)
		middleware.RecordFailedAttempt(h.db, ip)

		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(LoginResponse{
			Success: false,
			Message: "无效的令牌",
		})
		return
	}

	// 检查是否需要配置 R2
	var r2Configured string
	err := h.db.QueryRow("SELECT value FROM system_config WHERE key = 'r2_configured'").Scan(&r2Configured)
	needSetup := err == sql.ErrNoRows || r2Configured != "true"

	// 设置 Cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    req.Token,
		Path:     "/",
		MaxAge:   86400 * 7, // 7 天
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LoginResponse{
		Success:   true,
		NeedSetup: needSetup,
	})
}

// Status 获取认证状态
func (h *AuthHandler) Status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"方法不允许"}`, http.StatusMethodNotAllowed)
		return
	}

	// 检查是否需要配置 R2
	var r2Configured string
	err := h.db.QueryRow("SELECT value FROM system_config WHERE key = 'r2_configured'").Scan(&r2Configured)
	needSetup := err == sql.ErrNoRows || r2Configured != "true"

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"authenticated": true,
		"need_setup":    needSetup,
	})
}

// getClientIP 获取客户端 IP（复用 middleware 的逻辑）
func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	return r.RemoteAddr
}
