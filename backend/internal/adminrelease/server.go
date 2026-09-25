// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package adminrelease

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Server struct {
	cfg    Config
	db     *gorm.DB
	router *gin.Engine
	login  *loginLimiter
}

func NewServer(cfg Config) (*Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	db, err := OpenDatabase(cfg.MySQL)
	if err != nil {
		return nil, err
	}
	server := &Server{cfg: cfg, db: db, login: newLoginLimiter()}
	for _, directory := range []string{cfg.PublishRoot, cfg.DesktopPublishDir, cfg.AndroidPublishDir} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			_ = server.Close()
			return nil, err
		}
	}
	generatedPassword, err := server.bootstrapAdmin()
	if err != nil {
		_ = server.Close()
		return nil, err
	}
	if generatedPassword != "" {
		log.Printf("首次启动管理员账号: %s", cfg.BootstrapUsername)
		log.Printf("首次启动管理员密码: %s", generatedPassword)
		log.Printf("首次登录后请立即修改密码并安全保存")
	}
	server.router = server.buildRouter()
	return server, nil
}

func (s *Server) buildRouter() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(s.corsMiddleware())
	router.Use(func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "same-origin")
		c.Next()
	})
	router.GET("/livez", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "alive", "time": time.Now()})
	})
	router.GET("/readyz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()
		if err := s.checkDatabase(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable", "reason": "mysql unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready", "database": "mysql", "time": time.Now()})
	})
	router.POST("/api/v1/admin/auth/login", s.handleLogin)
	protected := router.Group("/api/v1/admin")
	protected.Use(s.authMiddleware())
	protected.Use(requireMutationToken)
	protected.GET("/auth/me", s.handleMe)
	protected.POST("/auth/logout", s.handleLogout)
	protected.POST("/auth/password", requireAdmin, s.handleChangePassword)
	protected.GET("/overview", s.handleOverview)
	protected.GET("/releases", s.handleListReleases)
	protected.POST("/releases", requirePublisher, s.handleCreateRelease)
	protected.GET("/releases/:id", s.handleGetRelease)
	protected.DELETE("/releases/:id", requirePublisher, s.handleDeleteRelease)
	protected.POST("/releases/:id/artifacts", requirePublisher, s.handleUploadArtifact)
	protected.POST("/releases/:id/validate", requirePublisher, s.handleValidateRelease)
	protected.POST("/releases/:id/publish", requirePublisher, s.handlePublishRelease)
	protected.POST("/releases/:id/rollout", requirePublisher, s.handlePauseRelease)
	protected.POST("/releases/:id/rollback", requirePublisher, s.handleRollbackDesktop)
	protected.GET("/audit-logs", s.handleAuditLogs)
	protected.GET("/settings", requireAdmin, s.handleSettings)
	s.registerWebRoutes(router)
	return router
}

func (s *Server) corsMiddleware() gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, origin := range s.cfg.AllowedOrigins {
		allowed[strings.TrimRight(origin, "/")] = true
	}
	return func(c *gin.Context) {
		origin := strings.TrimRight(c.GetHeader("Origin"), "/")
		if origin != "" && allowed[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token")
			c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			c.Header("Vary", "Origin")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func (s *Server) registerWebRoutes(router *gin.Engine) {
	indexPath := filepath.Join(s.cfg.AdminWebDir, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		return
	}
	router.Static("/assets", filepath.Join(s.cfg.AdminWebDir, "assets"))
	router.StaticFile("/favicon.ico", filepath.Join(s.cfg.AdminWebDir, "favicon.ico"))
	router.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "接口不存在", "data": nil})
			return
		}
		c.File(indexPath)
	})
}

func (s *Server) Router() http.Handler {
	return s.router
}

func (s *Server) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
