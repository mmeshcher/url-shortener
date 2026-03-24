package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/mmeshcher/url-shortener/internal/audit"
	"github.com/mmeshcher/url-shortener/internal/middleware"
	"go.uber.org/zap"
)

// RedirectHandler handles GET requests to redirect a short URL to its original long URL.
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

	userID, _ := middleware.GetUserIDFromContext(r.Context())
	h.auditor.Audit(audit.ActionFollow, userID, originalURL)

	rw.Header().Set("Location", originalURL)
	rw.WriteHeader(http.StatusTemporaryRedirect)
}
