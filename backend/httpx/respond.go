package httpx

import (
	"encoding/json"
	"net/http"
)

// WriteJSONError 输出统一格式的 JSON 错误
func WriteJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
