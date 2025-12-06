package main

import (
	"net/http"

	"github.com/mmeshcher/url-shortener/internal/handler"
	"github.com/mmeshcher/url-shortener/internal/service"
)

func main() {
	shortnerService := service.NewShortenerService()

	h := handler.NewHandler(shortnerService)

	r := h.SetupRouter()

	http.ListenAndServe(":8080", r)
}
