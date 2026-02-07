package handler

import (
	"errors"
	"io"
	"net/http"

	"github.com/mmeshcher/url-shortener/internal/middleware"
	"github.com/mmeshcher/url-shortener/internal/service"
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
	ctx := r.Context()
	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok || userID == "" {
		http.Error(rw, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	shortURL, err := h.service.CreateShortURL(r.Context(), originalURL, userID)

	if err != nil {
		if errors.Is(err, service.ErrURLAlreadyExists) {
			rw.Header().Set("Content-Type", "text/plain")
			rw.WriteHeader(http.StatusConflict)
			rw.Write([]byte(shortURL))
			return
		}

		if errors.Is(err, service.ErrEmptyURL) || errors.Is(err, service.ErrInvalidURL) {
			http.Error(rw, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		h.logger.Error("Failed to create short URL", zap.Error(err))
		http.Error(rw, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	rw.Header().Set("Content-Type", "text/plain")
	rw.WriteHeader(http.StatusCreated)
	rw.Write([]byte(shortURL))
}
