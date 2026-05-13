package handler

import (
	"encoding/json"
	"net/http"

	"github.com/mmeshcher/url-shortener/internal/models/domain"
	"go.uber.org/zap"
)

// StatsHandler returns system statistics including the number of URLs and users.
func (h *Handler) StatsHandler(w http.ResponseWriter, r *http.Request) {
	urlsCount, usersCount, err := h.service.GetStats(r.Context())
	if err != nil {
		h.logger.Error("Failed to get system statistics", zap.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	resp := domain.StatsResponse{
		URLs:  urlsCount,
		Users: usersCount,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("Failed to encode statistics response", zap.Error(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}
