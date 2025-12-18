package handlers

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"r2box/database"
	"r2box/middleware"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	db *sql.DB
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(db *sql.DB) *AuthHandler {
	return &AuthHandler{db: db}
}

// LoginRequest 登录请求
type LoginRequest struct {
	Password string `json:"password"`
}

// SetupPasswordRequest 设置密码请求
type SetupPasswordRequest struct {
	Password string `json:"password"`
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

	// 获取存储的密码哈希
	storedHash, err := database.GetPasswordHash()
	if err != nil || storedHash == "" {
		http.Error(w, `{"error":"密码未设置"}`, http.StatusInternalServerError)
		return
	}

	// 验证密码
	inputHash := hashPassword(req.Password)
	if inputHash != storedHash {
		ip := getClientIP(r)
		middleware.RecordFailedAttempt(h.db, ip)

		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "密码错误",
		})
		return
	}

	// 检查是否需要配置 R2
	needSetup := !checkR2Configured(h.db)

	// 设置 Cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    inputHash,
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":    true,
		"need_setup": needSetup,
	})
}

// SetupPassword 首次设置密码
func (h *AuthHandler) SetupPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"方法不允许"}`, http.StatusMethodNotAllowed)
		return
	}

	// 检查密码是否已设置
	if database.IsPasswordSet() {
		http.Error(w, `{"error":"密码已设置"}`, http.StatusBadRequest)
		return
	}

	var req SetupPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"无效的请求"}`, http.StatusBadRequest)
		return
	}


	// 保存密码哈希
	hash := hashPassword(req.Password)
	if err := database.SetPasswordHash(hash); err != nil {
		http.Error(w, `{"error":"保存密码失败"}`, http.StatusInternalServerError)
		return
	}

	// 设置 Cookie 自动登录
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    hash,
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":    true,
		"need_setup": true,
	})
}

// CheckPasswordStatus 检查密码状态
func (h *AuthHandler) CheckPasswordStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"方法不允许"}`, http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"password_set": database.IsPasswordSet(),
	})
}

// Status 获取认证状态
func (h *AuthHandler) Status(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"方法不允许"}`, http.StatusMethodNotAllowed)
		return
	}

	needSetup := !checkR2Configured(h.db)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"authenticated": true,
		"need_setup":    needSetup,
	})
}

func checkR2Configured(db *sql.DB) bool {
	var r2Configured string
	err := db.QueryRow("SELECT value FROM system_config WHERE key = 'r2_configured'").Scan(&r2Configured)
	return err == nil && r2Configured == "true"
}

func hashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	return r.RemoteAddr
}
