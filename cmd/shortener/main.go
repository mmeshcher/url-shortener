package main

import (
	"net/http"

	"go.uber.org/zap"

	"github.com/mmeshcher/url-shortener/internal/config"
	"github.com/mmeshcher/url-shortener/internal/handler"
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
	)
	shortnerService := service.NewShortenerService(cfg.BaseURL, cfg.FileStoragePath, logger)

	h := handler.NewHandler(shortnerService, logger)

	r := h.SetupRouter()

	sugar.Infow(
		"Server starting",
		"address", cfg.ServerAddress,
	)

	if err := http.ListenAndServe(cfg.ServerAddress, r); err != nil {
		sugar.Fatalw(err.Error(), "event", "start server")
	}
}
