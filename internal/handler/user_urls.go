package handler

import (
	"encoding/json"
	"net/http"

	"github.com/mmeshcher/url-shortener/internal/middleware"
	"go.uber.org/zap"
)

func (h *Handler) GetUserURLsHandler(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok || userID == "" {
		http.Error(rw, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	userURLs, err := h.service.GetUserURLs(ctx, userID)
	if err != nil {
		h.logger.Error("Failed to get user URLs",
			zap.String("userID", userID),
			zap.Error(err))
		http.Error(rw, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if len(userURLs) == 0 {
		rw.WriteHeader(http.StatusNoContent)
		return
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusOK)

	encoder := json.NewEncoder(rw)
	if err := encoder.Encode(userURLs); err != nil {
		h.logger.Error("Failed to encode user URLs response", zap.Error(err))
		http.Error(rw, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}
