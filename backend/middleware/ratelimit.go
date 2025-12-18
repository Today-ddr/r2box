package middleware

import (
	"database/sql"
	"net"
	"net/http"
	"time"
)

const (
	RateLimitWindow   = 1 * time.Minute
	RateLimitMax      = 300
	MaxFailedAttempts = 10
	BlockDuration     = 5 * time.Minute
)

// RateLimitMiddleware 速率限制中间件
func RateLimitMiddleware(db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getClientIP(r)

			// 检查是否被锁定
			if isBlocked(db, ip) {
				http.Error(w, `{"error":"请求过于频繁，请稍后再试"}`, http.StatusTooManyRequests)
				return
			}

			// 检查请求频率
			if !allowRequest(db, ip) {
				http.Error(w, `{"error":"请求频率超限"}`, http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// getClientIP 获取客户端 IP
func getClientIP(r *http.Request) string {
	// 尝试从 X-Forwarded-For 获取
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := parseXFF(xff)
		if len(ips) > 0 {
			return ips[0]
		}
	}

	// 尝试从 X-Real-IP 获取
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// 从 RemoteAddr 获取
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}

// parseXFF 解析 X-Forwarded-For header
func parseXFF(xff string) []string {
	var ips []string
	for _, ip := range splitAndTrim(xff, ",") {
		if ip != "" {
			ips = append(ips, ip)
		}
	}
	return ips
}

// splitAndTrim 分割并去除空格
func splitAndTrim(s, sep string) []string {
	var result []string
	for _, item := range split(s, sep) {
		trimmed := trim(item)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func split(s, sep string) []string {
	// 简单实现，实际应使用 strings.Split
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep[0] {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}

func trim(s string) string {
	// 简单实现，实际应使用 strings.TrimSpace
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

// isBlocked 检查 IP 是否被锁定
func isBlocked(db *sql.DB, ip string) bool {
	var blockedUntil sql.NullTime
	err := db.QueryRow("SELECT blocked_until FROM rate_limits WHERE ip = ?", ip).Scan(&blockedUntil)

	if err != nil || !blockedUntil.Valid {
		return false
	}

	if time.Now().Before(blockedUntil.Time) {
		return true
	}

	// 锁定已过期，清除锁定状态
	db.Exec("UPDATE rate_limits SET blocked_until = NULL, failed_attempts = 0 WHERE ip = ?", ip)
	return false
}

// allowRequest 检查是否允许请求
func allowRequest(db *sql.DB, ip string) bool {
	var requestCount int
	var windowStart time.Time

	err := db.QueryRow("SELECT request_count, window_start FROM rate_limits WHERE ip = ?", ip).Scan(&requestCount, &windowStart)

	now := time.Now()

	if err == sql.ErrNoRows {
		// 首次请求
		db.Exec("INSERT INTO rate_limits (ip, request_count, window_start) VALUES (?, 1, ?)", ip, now)
		return true
	}

	// 检查时间窗口是否过期
	if now.Sub(windowStart) > RateLimitWindow {
		// 重置计数器
		db.Exec("UPDATE rate_limits SET request_count = 1, window_start = ? WHERE ip = ?", now, ip)
		return true
	}

	// 检查是否超过限制
	if requestCount >= RateLimitMax {
		return false
	}

	// 增加计数器
	db.Exec("UPDATE rate_limits SET request_count = request_count + 1 WHERE ip = ?", ip)
	return true
}

// RecordFailedAttempt 记录失败尝试
func RecordFailedAttempt(db *sql.DB, ip string) {
	var failedAttempts int
	err := db.QueryRow("SELECT failed_attempts FROM rate_limits WHERE ip = ?", ip).Scan(&failedAttempts)

	if err == sql.ErrNoRows {
		db.Exec("INSERT INTO rate_limits (ip, failed_attempts) VALUES (?, 1)", ip)
		return
	}

	failedAttempts++

	if failedAttempts >= MaxFailedAttempts {
		// 锁定 IP
		blockedUntil := time.Now().Add(BlockDuration)
		db.Exec("UPDATE rate_limits SET failed_attempts = ?, blocked_until = ? WHERE ip = ?", failedAttempts, blockedUntil, ip)
	} else {
		db.Exec("UPDATE rate_limits SET failed_attempts = ? WHERE ip = ?", failedAttempts, ip)
	}
}
