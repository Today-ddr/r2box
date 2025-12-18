package middleware

import (
	"net/http"
	"r2box/database"
	"strings"
)

// AuthMiddleware 认证中间件
func AuthMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 获取存储的密码哈希
			storedHash, err := database.GetPasswordHash()
			if err != nil || storedHash == "" {
				http.Error(w, `{"error":"密码未设置"}`, http.StatusUnauthorized)
				return
			}

			// 从 Authorization header 获取 token
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				// 尝试从 Cookie 获取
				cookie, err := r.Cookie("auth_token")
				if err != nil {
					http.Error(w, `{"error":"未授权"}`, http.StatusUnauthorized)
					return
				}
				authHeader = "Bearer " + cookie.Value
			}

			// 验证 Bearer token
			if !strings.HasPrefix(authHeader, "Bearer ") {
				http.Error(w, `{"error":"无效的认证格式"}`, http.StatusUnauthorized)
				return
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token != storedHash {
				http.Error(w, `{"error":"无效的令牌"}`, http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
