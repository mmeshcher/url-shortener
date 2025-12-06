package main

import (
	"net/http"

	"github.com/mmeshcher/url-shortener/internal/handler"
	"github.com/mmeshcher/url-shortener/internal/service"
)

func main() {
	shortnerService := service.NewShortenerService()

	h := handler.NewHandler(shortnerService)

	http.HandleFunc("/", h.HandleRequest)

	http.ListenAndServe(":8080", nil)
}
