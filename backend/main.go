package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"r2box/config"
	"r2box/database"
	"r2box/handlers"
	"r2box/middleware"
	"r2box/models"
	"r2box/services"
	"sync"
	"time"
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

// StartCleanupTask 启动过期文件清理任务
func (a *App) StartCleanupTask() {
	cleanup := func() {
		r2Service := a.GetR2Service()
		if r2Service == nil {
			return
		}

		files, err := models.GetExpiredFiles(database.DB)
		if err != nil {
			log.Printf("[Cleanup] 获取过期文件失败: %v", err)
			return
		}

		for _, file := range files {
			// 删除 R2 对象
			if err := r2Service.DeleteObject(file.R2Key); err != nil {
				log.Printf("[Cleanup] 删除 R2 对象失败: %s, %v", file.R2Key, err)
				continue
			}
			// 标记为已删除（保留记录）
			if err := file.UpdateStatus(database.DB, "deleted"); err != nil {
				log.Printf("[Cleanup] 更新状态失败: %s, %v", file.ID, err)
				continue
			}
			log.Printf("[Cleanup] 已清理过期文件: %s (%s)", file.Filename, file.ID)
		}
	}

	go func() {
		// 启动时立即执行一次清理
		cleanup()

		ticker := time.NewTicker(1 * time.Hour) // 每小时检查一次
		defer ticker.Stop()

		for range ticker.C {
			cleanup()
		}
	}()
}

func main() {
	log.Printf("[App] R2Box v%s (%s) 启动中...", Version, CommitSHA)

	// 加载配置
	cfg := config.Load()

	// 初始化数据库
	if err := database.Init(cfg.DatabasePath); err != nil {
		log.Fatalf("[App] 数据库初始化失败: %v", err)
	}
	defer database.Close()

	log.Println("[App] 数据库初始化成功")

	// 创建应用实例
	app := &App{cfg: cfg}

	// 尝试初始化 R2 服务
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

	// 创建路由器
	mux := http.NewServeMux()

	// 创建认证处理器
	authHandler := handlers.NewAuthHandler(database.DB)

	// 公开路由（无需认证）
	mux.HandleFunc("/api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[API] POST /api/auth/login")
		authHandler.Login(w, r)
	})

	mux.HandleFunc("/api/auth/setup-password", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[API] POST /api/auth/setup-password")
		authHandler.SetupPassword(w, r)
	})

	mux.HandleFunc("/api/auth/password-status", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[API] GET /api/auth/password-status")
		authHandler.CheckPasswordStatus(w, r)
	})

	// R2 配置向导（带配置变更回调）
	setupHandler := handlers.NewSetupHandler(database.DB, func() {
		app.ReloadR2Service()
	})

	// 认证状态
	mux.Handle("/api/auth/status", middleware.AuthMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[API] GET /api/auth/status")
		authHandler.Status(w, r)
	})))

	// R2 配置向导路由
	mux.Handle("/api/setup/status", middleware.AuthMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[API] GET /api/setup/status")
		setupHandler.Status(w, r)
	})))

	mux.Handle("/api/setup/config", middleware.AuthMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[API] POST /api/setup/config")
		setupHandler.SaveConfig(w, r)
	})))

	mux.Handle("/api/setup/test", middleware.AuthMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[API] POST /api/setup/test")
		setupHandler.TestConnection(w, r)
	})))

	// 上传路由（动态获取 R2 服务）
	mux.Handle("/api/upload/presign", middleware.AuthMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[API] POST /api/upload/presign")
		r2Service := app.GetR2Service()
		if r2Service == nil {
			log.Println("[API] R2 服务未初始化")
			http.Error(w, `{"error":"R2 未配置，请先完成配置"}`, http.StatusServiceUnavailable)
			return
		}
		uploadHandler := handlers.NewUploadHandler(database.DB, r2Service, cfg.MaxFileSize)
		uploadHandler.GeneratePresignURL(w, r)
	})))

	mux.Handle("/api/upload/multipart/init", middleware.AuthMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[API] POST /api/upload/multipart/init")
		r2Service := app.GetR2Service()
		if r2Service == nil {
			http.Error(w, `{"error":"R2 未配置，请先完成配置"}`, http.StatusServiceUnavailable)
			return
		}
		uploadHandler := handlers.NewUploadHandler(database.DB, r2Service, cfg.MaxFileSize)
		uploadHandler.InitiateMultipartUpload(w, r)
	})))

	mux.Handle("/api/upload/multipart/presign", middleware.AuthMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[API] POST /api/upload/multipart/presign")
		r2Service := app.GetR2Service()
		if r2Service == nil {
			http.Error(w, `{"error":"R2 未配置，请先完成配置"}`, http.StatusServiceUnavailable)
			return
		}
		uploadHandler := handlers.NewUploadHandler(database.DB, r2Service, cfg.MaxFileSize)
		uploadHandler.GenerateMultipartPresignURL(w, r)
	})))

	mux.Handle("/api/upload/multipart/complete", middleware.AuthMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[API] POST /api/upload/multipart/complete")
		r2Service := app.GetR2Service()
		if r2Service == nil {
			http.Error(w, `{"error":"R2 未配置，请先完成配置"}`, http.StatusServiceUnavailable)
			return
		}
		uploadHandler := handlers.NewUploadHandler(database.DB, r2Service, cfg.MaxFileSize)
		uploadHandler.CompleteMultipartUpload(w, r)
	})))

	// 确认上传完成（小文件）
	mux.Handle("/api/upload/confirm", middleware.AuthMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[API] POST /api/upload/confirm")
		r2Service := app.GetR2Service()
		if r2Service == nil {
			http.Error(w, `{"error":"R2 未配置，请先完成配置"}`, http.StatusServiceUnavailable)
			return
		}
		uploadHandler := handlers.NewUploadHandler(database.DB, r2Service, cfg.MaxFileSize)
		uploadHandler.ConfirmUpload(w, r)
	})))

	// 取消上传
	mux.Handle("/api/upload/cancel", middleware.AuthMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[API] POST /api/upload/cancel")
		r2Service := app.GetR2Service()
		if r2Service == nil {
			http.Error(w, `{"error":"R2 未配置，请先完成配置"}`, http.StatusServiceUnavailable)
			return
		}
		uploadHandler := handlers.NewUploadHandler(database.DB, r2Service, cfg.MaxFileSize)
		uploadHandler.CancelUpload(w, r)
	})))

	// 文件列表路由
	mux.Handle("/api/files", middleware.AuthMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[API] GET /api/files")
		r2Service := app.GetR2Service()
		if r2Service == nil {
			http.Error(w, `{"error":"R2 未配置，请先完成配置"}`, http.StatusServiceUnavailable)
			return
		}
		filesHandler := handlers.NewFilesHandler(database.DB, r2Service)
		filesHandler.List(w, r)
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

		http.Redirect(w, r, "/api/files/"+file.ID+"/download", http.StatusFound)
	})

	// 文件下载和删除路由
	mux.HandleFunc("/api/files/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[API] %s /api/files/...", r.Method)
		r2Service := app.GetR2Service()
		if r2Service == nil {
			http.Error(w, `{"error":"R2 未配置，请先完成配置"}`, http.StatusServiceUnavailable)
			return
		}
		filesHandler := handlers.NewFilesHandler(database.DB, r2Service)

		if r.Method == http.MethodDelete {
			// 删除需要认证
			middleware.AuthMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				filesHandler.Delete(w, r)
			})).ServeHTTP(w, r)
		} else {
			// 下载不需要认证
			filesHandler.GetDownloadURL(w, r)
		}
	})

	// 存储统计路由
	mux.Handle("/api/stats", middleware.AuthMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[API] GET /api/stats")
		statsHandler := handlers.NewStatsHandler(database.DB, cfg.TotalStorage)
		statsHandler.GetStats(w, r)
	})))

	// 静态文件服务（前端）
	staticDir := "./static"
	if _, err := os.Stat(staticDir); err == nil {
		fs := http.FileServer(http.Dir(staticDir))
		mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 如果请求的是 API 路径，返回 404
			if len(r.URL.Path) >= 4 && r.URL.Path[:4] == "/api" {
				http.NotFound(w, r)
				return
			}

			// 检查文件是否存在
			path := filepath.Join(staticDir, r.URL.Path)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				// 如果文件不存在，返回 index.html（用于 SPA 路由）
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

	// 应用速率限制中间件
	handler := middleware.RateLimitMiddleware(database.DB)(mux)

	// 启动服务器
	addr := ":" + cfg.Port
	// 启动过期文件清理任务
	app.StartCleanupTask()
	log.Println("[App] 过期文件清理任务已启动")

	passwordSet := database.IsPasswordSet()
	log.Printf("[App] ========================================")
	log.Printf("[App] R2Box 服务器启动成功")
	log.Printf("[App] 地址: http://localhost%s", addr)
	log.Printf("[App] 密码状态: %v", passwordSet)
	log.Printf("[App] R2 配置状态: %v", r2Configured)
	log.Printf("[App] ========================================")

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("[App] 服务器启动失败: %v", err)
	}
}
