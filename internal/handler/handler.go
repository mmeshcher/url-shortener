package handler

import (
	"net/http"

	"github.com/mmeshcher/url-shortener/internal/service"
)

type Handler struct {
	service *service.ShortnerService
}

func NewHandler(service *service.ShortnerService) *Handler {
	return &Handler{
		service: service,
	}
}

func (h *Handler) HandleRequest(w http.ResponseWriter, r *http.Request) {
	switch {
	// POST / - создание короткой ссылки
	case r.Method == http.MethodPost && r.URL.Path == "/":
		h.ShortenHandler(w, r)

	// GET /{id} - редирект
	case r.Method == http.MethodGet && r.URL.Path != "/":
		h.RedirectHandler(w, r)

	// Все остальные случаи - 400 Bad Request
	default:
		http.Error(w, "Bad Request", http.StatusBadRequest)
	}
}
