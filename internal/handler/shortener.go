// internal/handler/shortener.go
package handler

import (
	"io"
	"net/http"

	"go.uber.org/zap"
)

func (h *Handler) ShortenHandler(rw http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		http.Error(rw, "Empty body", http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	originalURL := string(body)
	shortURL, conflict, err := h.service.CreateShortURL(originalURL)

	if err != nil && shortURL == "" {
		h.logger.Error("Failed to create short URL", zap.Error(err))
		http.Error(rw, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if conflict {
		rw.Header().Set("Content-Type", "text/plain")
		rw.WriteHeader(http.StatusConflict)
		rw.Write([]byte(shortURL))
		return
	}

	if shortURL == "" {
		h.logger.Error("Failed to create short URL (empty result returned)")
		http.Error(rw, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", "text/plain")
	rw.WriteHeader(http.StatusCreated)
	rw.Write([]byte(shortURL))
}
