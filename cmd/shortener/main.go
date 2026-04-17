// Package main is the entry point for the URL shortener server application.
// It initializes configuration, logging, auditing, and starts the HTTP server.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/mmeshcher/url-shortener/internal/audit"
	"github.com/mmeshcher/url-shortener/internal/cert"
	"github.com/mmeshcher/url-shortener/internal/config"
	"github.com/mmeshcher/url-shortener/internal/handler"
	"github.com/mmeshcher/url-shortener/internal/middleware"
	"github.com/mmeshcher/url-shortener/internal/service"
)

var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

func main() {
	fmt.Printf("Build version: %s\n", getBuildValue(buildVersion))
	fmt.Printf("Build date: %s\n", getBuildValue(buildDate))
	fmt.Printf("Build commit: %s\n", getBuildValue(buildCommit))

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
			"https", cfg.EnableHTTPS,
		)

		if cfg.EnableHTTPS {
			certFile := "server.crt"
			keyFile := "server.key"

			// Check if certificate files exist, if not generate them
			if _, err := os.Stat(certFile); os.IsNotExist(err) {
				if err := cert.GenerateSelfSignedCert(certFile, keyFile); err != nil {
					sugar.Fatalw("Failed to generate self-signed certificate", "error", err)
				}
				sugar.Infow("Self-signed certificate generated", "cert", certFile, "key", keyFile)
			}

			if err := server.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
				sugar.Fatalw("HTTPS server failed", "error", err.Error())
			}
		} else {
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				sugar.Fatalw("HTTP server failed", "error", err.Error())
			}
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	<-quit

	sugar.Info("Shutting down server...")

	// Gracefully shutdown HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		sugar.Errorw("Server shutdown failed",
			"error", err.Error())
	}

	// Service.Close() is called via defer at the beginning of main()
	sugar.Info("Server stopped")
}

func getBuildValue(v string) string {
	if v == "" {
		return "N/A"
	}
	return v
}
