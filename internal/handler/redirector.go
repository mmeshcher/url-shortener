package handler

import (
	"net/http"
)

func (h *Handler) RedirectHandler(rw http.ResponseWriter, r *http.Request) {
	shortUrl := r.URL.Path[1:]
	if shortUrl == "" {
		http.Error(rw, "Empty short url", http.StatusBadRequest)
		return
	}

	originalUrl, exists := h.service.GetOriginalUrl(shortUrl)
	if !exists {
		http.Error(rw, "Original URL not exists for this short URL", http.StatusBadRequest)
		return
	}

	rw.Header().Set("Location", originalUrl)
	rw.WriteHeader(http.StatusTemporaryRedirect)
}
