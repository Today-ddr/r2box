package middleware

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"r2box/httpx"
)

const (
	RateLimitWindow   = 1 * time.Minute
	RateLimitMax      = 300
	MaxFailedAttempts = 10
	BlockDuration     = 5 * time.Minute
)

// trustedProxies 只有来自这些网段的请求才允许提供 X-Forwarded-For
var trustedProxies []*net.IPNet

// InitTrustedProxies 解析受信代理网段。为空时一律使用 RemoteAddr。
func InitTrustedProxies(cidrs []string) error {
	trustedProxies = nil
	for _, raw := range cidrs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if !strings.Contains(raw, "/") {
			if ip := net.ParseIP(raw); ip != nil {
				if ip.To4() != nil {
					raw += "/32"
				} else {
					raw += "/128"
				}
			}
		}
		_, network, err := net.ParseCIDR(raw)
		if err != nil {
			return fmt.Errorf("无效的 TRUSTED_PROXIES 网段 %q: %w", raw, err)
		}
		trustedProxies = append(trustedProxies, network)
	}
	if len(trustedProxies) > 0 {
		log.Printf("[RateLimit] 已信任 %d 个代理网段", len(trustedProxies))
	}
	return nil
}

func isTrusted(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, network := range trustedProxies {
		if network.Contains(parsed) {
			return true
		}
	}
	return false
}

// GetClientIP 获取客户端 IP。仅当直连方是受信代理时才解析 X-Forwarded-For，
// 并从右往左取第一个非受信地址，避免客户端伪造头部绕过限流。
func GetClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}

	if !isTrusted(host) {
		return host
	}

	xff := r.Header.Get("X-Forwarded-For")
	if xff == "" {
		return host
	}

	parts := strings.Split(xff, ",")
	for i := len(parts) - 1; i >= 0; i-- {
		ip := strings.TrimSpace(parts[i])
		if ip == "" {
			continue
		}
		if !isTrusted(ip) {
			return ip
		}
	}

	return host
}

// RateLimitMiddleware 速率限制中间件
func RateLimitMiddleware(db *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := GetClientIP(r)

			if isBlocked(db, ip) {
				httpx.WriteJSONError(w, http.StatusTooManyRequests, "请求过于频繁，请稍后再试")
				return
			}

			if !allowRequest(db, ip) {
				httpx.WriteJSONError(w, http.StatusTooManyRequests, "请求频率超限")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isBlocked 检查 IP 是否被锁定
func isBlocked(db *sql.DB, ip string) bool {
	var blockedUntil sql.NullInt64
	if err := db.QueryRow("SELECT blocked_until_unix FROM rate_limits WHERE ip = ?", ip).Scan(&blockedUntil); err != nil {
		return false
	}

	if !blockedUntil.Valid {
		return false
	}

	if time.Now().Unix() < blockedUntil.Int64 {
		return true
	}

	db.Exec("UPDATE rate_limits SET blocked_until_unix = NULL, failed_attempts = 0 WHERE ip = ?", ip)
	return false
}

// allowRequest 检查并累加请求计数，读写合并为一条语句
func allowRequest(db *sql.DB, ip string) bool {
	now := time.Now().Unix()
	window := int64(RateLimitWindow.Seconds())

	var count int
	err := db.QueryRow(`
		INSERT INTO rate_limits (ip, request_count, window_start_unix, failed_attempts)
		VALUES (?, 1, ?, 0)
		ON CONFLICT(ip) DO UPDATE SET
			request_count = CASE WHEN ? - window_start_unix >= ? THEN 1 ELSE request_count + 1 END,
			window_start_unix = CASE WHEN ? - window_start_unix >= ? THEN ? ELSE window_start_unix END
		RETURNING request_count
	`, ip, now, now, window, now, window, now).Scan(&count)
	if err != nil {
		// 限流自身出错时不阻断业务
		log.Printf("[RateLimit] 计数失败: %v", err)
		return true
	}

	return count <= RateLimitMax
}

// RecordFailedAttempt 记录失败尝试
func RecordFailedAttempt(db *sql.DB, ip string) {
	now := time.Now().Unix()

	var failedAttempts int
	err := db.QueryRow(`
		INSERT INTO rate_limits (ip, request_count, window_start_unix, failed_attempts)
		VALUES (?, 0, ?, 1)
		ON CONFLICT(ip) DO UPDATE SET failed_attempts = failed_attempts + 1
		RETURNING failed_attempts
	`, ip, now).Scan(&failedAttempts)
	if err != nil {
		log.Printf("[RateLimit] 记录失败尝试出错: %v", err)
		return
	}

	if failedAttempts >= MaxFailedAttempts {
		blockedUntil := time.Now().Add(BlockDuration).Unix()
		db.Exec("UPDATE rate_limits SET blocked_until_unix = ? WHERE ip = ?", blockedUntil, ip)
	}
}

// CleanupRateLimits 清理长时间无活动且未被锁定的限流记录
func CleanupRateLimits(db *sql.DB, before time.Time) error {
	_, err := db.Exec(`
		DELETE FROM rate_limits
		WHERE window_start_unix < ?
		  AND (blocked_until_unix IS NULL OR blocked_until_unix < ?)
	`, before.Unix(), time.Now().Unix())
	return err
}
