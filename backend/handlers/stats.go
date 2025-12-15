package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"r2box/models"
)

// StatsHandler 存储统计处理器
type StatsHandler struct {
	db           *sql.DB
	totalStorage int64
}

// NewStatsHandler 创建存储统计处理器
func NewStatsHandler(db *sql.DB, totalStorage int64) *StatsHandler {
	return &StatsHandler{
		db:           db,
		totalStorage: totalStorage,
	}
}

// GetStats 获取存储统计
func (h *StatsHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"方法不允许"}`, http.StatusMethodNotAllowed)
		return
	}

	stats, err := models.GetStorageStats(h.db, h.totalStorage)
	if err != nil {
		http.Error(w, `{"error":"获取存储统计失败"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
