package middleware

import (
	"net/http"
	"strings"
)

// AuthMiddleware 认证中间件
func AuthMiddleware(accessToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			if token != accessToken {
				http.Error(w, `{"error":"无效的令牌"}`, http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
