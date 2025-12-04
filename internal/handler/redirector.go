package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) RedirectHandler(rw http.ResponseWriter, r *http.Request) {
	shortURL := chi.URLParam(r, "shortID")
	if shortURL == "" {
		http.Error(rw, "Bad Request", http.StatusBadRequest)
		return
	}

	originalURL, exists := h.service.GetOriginalURL(shortURL)
	if !exists {
		http.Error(rw, "Not Found", http.StatusNotFound)
		return
	}

	rw.Header().Set("Location", originalURL)
	rw.WriteHeader(http.StatusTemporaryRedirect)
}
