package handler

import (
	"io"
	"net/http"
)

func (h *Handler) ShortenHandler(rw http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		http.Error(rw, "Empty body", http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	originalURL := string(body)
	shortURL := h.service.CreateShortURL(originalURL)

	rw.Header().Set("Content-Type", "text/plain")
	rw.WriteHeader(http.StatusCreated)
	rw.Write([]byte(shortURL))
}
