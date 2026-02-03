package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

func (h *Handler) RedirectHandler(rw http.ResponseWriter, r *http.Request) {
	shortURL := chi.URLParam(r, "shortID")
	if shortURL == "" {
		http.Error(rw, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	originalURL, exists, deleted := h.service.GetOriginalURL(shortURL)

	if deleted {
		h.logger.Info("Access to deleted URL",
			zap.String("shortID", shortURL))
		http.Error(rw, http.StatusText(http.StatusGone), http.StatusGone)
		return
	}

	if !exists {
		http.Error(rw, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	rw.Header().Set("Location", originalURL)
	rw.WriteHeader(http.StatusTemporaryRedirect)
}
