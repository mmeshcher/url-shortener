// Package main is the entry point for the URL shortener server application.
// It initializes configuration, logging, auditing, and starts the HTTP server.
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/mmeshcher/url-shortener/internal/audit"
	"github.com/mmeshcher/url-shortener/internal/config"
	"github.com/mmeshcher/url-shortener/internal/handler"
	"github.com/mmeshcher/url-shortener/internal/middleware"
	"github.com/mmeshcher/url-shortener/internal/service"
)

func main() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		logger.Fatal("Failed to create logger", zap.Error(err))
	}
	defer logger.Sync()

	sugar := logger.Sugar()

	sugar.Infow(
		"Starting URL shortener service",
	)

	cfg, err := config.ParseFlags()
	if err != nil {
		sugar.Fatalw("Configuration error",
			"error", err.Error())
	}

	sugar.Infow(
		"Configuration loaded",
		"server_address", cfg.ServerAddress,
		"base_url", cfg.BaseURL,
		"file_storage_path", cfg.FileStoragePath,
		"using_database", cfg.DatabaseDSN != "",
		"audit_file", cfg.AuditFile,
		"audit_url", cfg.AuditURL,
	)

	auditor := audit.NewAuditor()
	if cfg.AuditFile != "" {
		auditor.Register(audit.NewFileObserver(cfg.AuditFile))
		sugar.Infow("Audit file observer registered", "path", cfg.AuditFile)
	}
	if cfg.AuditURL != "" {
		auditor.Register(audit.NewURLObserver(cfg.AuditURL))
		sugar.Infow("Audit URL observer registered", "url", cfg.AuditURL)
	}

	authMiddleware := middleware.NewAuthMiddleware(cfg.SecretKey, logger)

	shortnerService := service.NewShortenerService(cfg.BaseURL, cfg.FileStoragePath, logger, cfg.DatabaseDSN)

	defer shortnerService.Close()

	h := handler.NewHandler(shortnerService, logger, authMiddleware, auditor)

	r := h.SetupRouter()

	server := &http.Server{
		Addr:    cfg.ServerAddress,
		Handler: r,
	}

	go func() {
		sugar.Infow(
			"Server starting",
			"address", cfg.ServerAddress,
		)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			sugar.Fatalw("Server failed",
				"error", err.Error())
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	sugar.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		sugar.Errorw("Server shutdown failed",
			"error", err.Error())
	}

	sugar.Info("Server stopped")
}
