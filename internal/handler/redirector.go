package handler

import (
	"net/http"
)

func (h *Handler) RedirectHandler(rw http.ResponseWriter, r *http.Request) {
	shortURL := r.URL.Path[1:]
	if shortURL == "" {
		http.Error(rw, "Empty short url", http.StatusBadRequest)
		return
	}

	originalURL, exists := h.service.GetOriginalURL(shortURL)
	if !exists {
		http.Error(rw, "Original URL not exists for this short URL", http.StatusBadRequest)
		return
	}

	rw.Header().Set("Location", originalURL)
	rw.WriteHeader(http.StatusTemporaryRedirect)
}
