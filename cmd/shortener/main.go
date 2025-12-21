package main

import (
	"log"
	"net/http"

	"github.com/mmeshcher/url-shortener/internal/config"
	"github.com/mmeshcher/url-shortener/internal/handler"
	"github.com/mmeshcher/url-shortener/internal/service"
)

func main() {
	cfg, err := config.ParseFlags()
	if err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	log.Printf("Starting server with config: address=%s, baseURL=%s",
		cfg.ServerAddress, cfg.BaseURL)
	shortnerService := service.NewShortenerService(cfg.BaseURL)

	h := handler.NewHandler(shortnerService)

	r := h.SetupRouter()

	log.Printf("Server starting on %s", cfg.ServerAddress)
	log.Fatal(http.ListenAndServe(cfg.ServerAddress, r))
}
