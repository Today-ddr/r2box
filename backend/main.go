package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"r2box/config"
	"r2box/database"
	"r2box/handlers"
	"r2box/httpx"
	"r2box/middleware"
	"r2box/models"
	"r2box/services"
)

// Version info (injected at build time via -ldflags)
var (
	Version   = "dev"
	CommitSHA = "unknown"
)

// App 应用实例
type App struct {
	cfg       *config.Config
	r2Service *services.R2Service
	mu        sync.RWMutex
}

// GetR2Service 获取 R2 服务（线程安全）
func (a *App) GetR2Service() *services.R2Service {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.r2Service
}

// ReloadR2Service 重新加载 R2 服务
func (a *App) ReloadR2Service() {
	a.mu.Lock()
	defer a.mu.Unlock()

	log.Println("[App] 重新加载 R2 服务...")

	r2Service, err := services.NewR2Service(database.DB)
	if err != nil {
		log.Printf("[App] R2 服务重新加载失败: %v", err)
		return
	}

	a.r2Service = r2Service
	log.Println("[App] R2 服务重新加载成功")
}

// StartCleanupTask 启动定期清理任务
func (a *App) StartCleanupTask(ctx context.Context) {
	go func() {
		a.runCleanup()

		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Println("[Cleanup] 清理任务已停止")
				return
			case <-ticker.C:
				a.runCleanup()
			}
		}
	}()
}

func (a *App) runCleanup() {
	r2Service := a.GetR2Service()
	if r2Service == nil {
		return
	}

	a.cleanupExpired(r2Service)
	a.cleanupStaleUploads(r2Service)

	if removed, err := models.DeleteStaleFailed(database.DB, time.Now().Add(-7*24*time.Hour)); err != nil {
		log.Printf("[Cleanup] 删除陈旧失败记录失败: %v", err)
	} else if removed > 0 {
		log.Printf("[Cleanup] 已删除 %d 条陈旧失败记录", removed)
	}

	if err := middleware.CleanupRateLimits(database.DB, time.Now().Add(-1*time.Hour)); err != nil {
		log.Printf("[Cleanup] 清理限流记录失败: %v", err)
	}
}

// cleanupExpired 删除已过期的对象，记录保留为 deleted
func (a *App) cleanupExpired(r2Service *services.R2Service) {
	files, err := models.GetExpiredFiles(database.DB)
	if err != nil {
		log.Printf("[Cleanup] 获取过期文件失败: %v", err)
		return
	}

	for _, file := range files {
		if err := r2Service.DeleteObject(file.R2Key); err != nil {
			log.Printf("[Cleanup] 删除 R2 对象失败: %s, %v", file.R2Key, err)
			continue
		}
		if err := file.UpdateStatus(database.DB, "deleted"); err != nil {
			log.Printf("[Cleanup] 更新状态失败: %s, %v", file.ID, err)
			continue
		}
		log.Printf("[Cleanup] 已清理过期文件: %s (%s)", file.Filename, file.ID)
	}
}

// cleanupStaleUploads 回收中断的上传：中止分片、删除残留对象、释放配额
func (a *App) cleanupStaleUploads(r2Service *services.R2Service) {
	// 基准是创建时间：中断的上传不应该占着配额等到文件过期
	files, err := models.GetStaleUploads(database.DB, time.Now().Add(-24*time.Hour))
	if err != nil {
		log.Printf("[Cleanup] 获取超时上传失败: %v", err)
		return
	}

	for _, file := range files {
		if file.UploadID != "" {
			if err := r2Service.AbortMultipartUpload(file.R2Key, file.UploadID); err != nil {
				log.Printf("[Cleanup] 终止分片上传失败: %s, %v", file.ID, err)
			}
		}
		if err := r2Service.DeleteObject(file.R2Key); err != nil {
			log.Printf("[Cleanup] 删除残留对象失败: %s, %v", file.ID, err)
		}
		if err := file.UpdateStatus(database.DB, "failed"); err != nil {
			log.Printf("[Cleanup] 标记超时上传失败: %s, %v", file.ID, err)
			continue
		}
		log.Printf("[Cleanup] 已回收中断上传: %s (%s)", file.Filename, file.ID)
	}
}

// healthz 健康检查：数据库可用即视为健康
func healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if err := database.DB.Ping(); err != nil {
		log.Printf("[Health] 数据库不可用: %v", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"status": "unhealthy"})
		return
	}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func main() {
	log.Printf("[App] R2Box v%s (%s) 启动中...", Version, CommitSHA)

	cfg := config.Load()

	if err := middleware.InitTrustedProxies(cfg.TrustedProxies); err != nil {
		log.Fatalf("[App] 初始化受信代理失败: %v", err)
	}

	if err := database.Init(cfg.DatabasePath); err != nil {
		log.Fatalf("[App] 数据库初始化失败: %v", err)
	}
	defer database.Close()

	log.Println("[App] 数据库初始化成功")

	app := &App{cfg: cfg}

	r2Configured, _ := database.IsR2Configured()
	if r2Configured {
		r2Service, err := services.NewR2Service(database.DB)
		if err != nil {
			log.Printf("[App] 警告: R2 服务初始化失败: %v", err)
		} else {
			app.r2Service = r2Service
			log.Println("[App] R2 服务初始化成功")
		}
	} else {
		log.Println("[App] R2 尚未配置，等待用户配置")
	}

	mux := http.NewServeMux()

	authHandler := handlers.NewAuthHandler(database.DB)

	// 公开路由（无需认证）
	mux.HandleFunc("/healthz", healthz)
	mux.HandleFunc("/api/auth/login", authHandler.Login)
	mux.HandleFunc("/api/auth/setup-password", authHandler.SetupPassword)
	mux.HandleFunc("/api/auth/password-status", authHandler.CheckPasswordStatus)

	// R2 配置向导（带配置变更回调）
	setupHandler := handlers.NewSetupHandler(database.DB, func() {
		app.ReloadR2Service()
	})

	mux.Handle("/api/auth/status", middleware.AuthMiddleware()(http.HandlerFunc(authHandler.Status)))
	mux.Handle("/api/setup/status", middleware.AuthMiddleware()(http.HandlerFunc(setupHandler.Status)))
	mux.Handle("/api/setup/config", middleware.AuthMiddleware()(http.HandlerFunc(setupHandler.SaveConfig)))
	mux.Handle("/api/setup/test", middleware.AuthMiddleware()(http.HandlerFunc(setupHandler.TestConnection)))

	// uploadHandler 每次请求按当前 R2 服务构造，支持配置热更新
	uploadHandler := func() *handlers.UploadHandler {
		r2Service := app.GetR2Service()
		if r2Service == nil {
			return nil
		}
		return handlers.NewUploadHandler(database.DB, r2Service, cfg.MaxFileSize, cfg.TotalStorage)
	}

	withR2 := func(fn func(h *handlers.UploadHandler, w http.ResponseWriter, r *http.Request)) http.Handler {
		return middleware.AuthMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := uploadHandler()
			if h == nil {
				httpx.WriteJSONError(w, http.StatusServiceUnavailable, "R2 未配置，请先完成配置")
				return
			}
			fn(h, w, r)
		}))
	}

	mux.Handle("/api/upload/presign", withR2(func(h *handlers.UploadHandler, w http.ResponseWriter, r *http.Request) {
		h.GeneratePresignURL(w, r)
	}))
	mux.Handle("/api/upload/multipart/init", withR2(func(h *handlers.UploadHandler, w http.ResponseWriter, r *http.Request) {
		h.InitiateMultipartUpload(w, r)
	}))
	mux.Handle("/api/upload/multipart/presign", withR2(func(h *handlers.UploadHandler, w http.ResponseWriter, r *http.Request) {
		h.GenerateMultipartPresignURL(w, r)
	}))
	mux.Handle("/api/upload/multipart/complete", withR2(func(h *handlers.UploadHandler, w http.ResponseWriter, r *http.Request) {
		h.CompleteMultipartUpload(w, r)
	}))
	mux.Handle("/api/upload/confirm", withR2(func(h *handlers.UploadHandler, w http.ResponseWriter, r *http.Request) {
		h.ConfirmUpload(w, r)
	}))
	mux.Handle("/api/upload/cancel", withR2(func(h *handlers.UploadHandler, w http.ResponseWriter, r *http.Request) {
		h.CancelUpload(w, r)
	}))

	// 文件列表路由
	mux.Handle("/api/files", middleware.AuthMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r2Service := app.GetR2Service()
		if r2Service == nil {
			httpx.WriteJSONError(w, http.StatusServiceUnavailable, "R2 未配置，请先完成配置")
			return
		}
		handlers.NewFilesHandler(database.DB, r2Service).List(w, r)
	})))

	// 短链接访问路由
	mux.HandleFunc("/s/", func(w http.ResponseWriter, r *http.Request) {
		shortCode := r.URL.Path[3:]
		if shortCode == "" {
			http.Error(w, "短码不能为空", http.StatusBadRequest)
			return
		}

		file, err := models.GetFileByShortCode(database.DB, shortCode)
		if err != nil {
			http.Error(w, "文件不存在", http.StatusNotFound)
			return
		}

		http.Redirect(w, r, handlers.DownloadPath(file.ID), http.StatusFound)
	})

	// 文件下载和删除路由
	mux.HandleFunc("/api/files/", func(w http.ResponseWriter, r *http.Request) {
		r2Service := app.GetR2Service()
		if r2Service == nil {
			httpx.WriteJSONError(w, http.StatusServiceUnavailable, "R2 未配置，请先完成配置")
			return
		}
		filesHandler := handlers.NewFilesHandler(database.DB, r2Service)

		if r.Method == http.MethodDelete {
			middleware.AuthMiddleware()(http.HandlerFunc(filesHandler.Delete)).ServeHTTP(w, r)
			return
		}
		filesHandler.GetDownloadURL(w, r)
	})

	// 存储统计路由
	mux.Handle("/api/stats", middleware.AuthMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		statsHandler := handlers.NewStatsHandler(database.DB, cfg.TotalStorage)
		statsHandler.GetStats(w, r)
	})))

	// 静态文件服务（前端）
	staticDir := "./static"
	if _, err := os.Stat(staticDir); err == nil {
		fs := http.FileServer(http.Dir(staticDir))
		mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(r.URL.Path) >= 4 && r.URL.Path[:4] == "/api" {
				http.NotFound(w, r)
				return
			}

			path := filepath.Join(staticDir, r.URL.Path)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
				return
			}

			fs.ServeHTTP(w, r)
		}))
	} else {
		log.Println("[App] 警告: 静态文件目录不存在，前端将无法访问")
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>R2Box</title>
</head>
<body>
    <h1>R2Box 后端运行中</h1>
    <p>前端尚未构建。请运行前端构建后重启服务。</p>
    <p>API 端点: <a href="/api/auth/login">/api/auth/login</a></p>
</body>
</html>
			`)
		})
	}

	handler := middleware.RateLimitMiddleware(database.DB)(mux)

	// 收到 SIGINT/SIGTERM 后停止后台任务并优雅关闭
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app.StartCleanupTask(ctx)
	log.Println("[App] 过期文件清理任务已启动")

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		log.Println("[App] 收到退出信号，正在关闭...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("[App] 优雅关闭超时: %v", err)
		}
	}()

	log.Printf("[App] ========================================")
	log.Printf("[App] R2Box 服务器启动成功")
	log.Printf("[App] 地址: http://localhost%s", srv.Addr)
	log.Printf("[App] 密码状态: %v", database.IsPasswordSet())
	log.Printf("[App] R2 配置状态: %v", r2Configured)
	log.Printf("[App] ========================================")

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("[App] 服务器启动失败: %v", err)
	}

	log.Println("[App] 已退出")
}
