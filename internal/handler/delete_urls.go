package handler

import (
	"encoding/json"
	"net/http"

	"github.com/mmeshcher/url-shortener/internal/middleware"
	"github.com/mmeshcher/url-shortener/internal/models"
	"go.uber.org/zap"
)

func (h *Handler) DeleteUserURLsHandler(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(rw, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		http.Error(rw, "Content-Type must be application/json", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok || userID == "" {
		http.Error(rw, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	var deleteReq models.DeleteRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&deleteReq); err != nil {
		h.logger.Error("Failed to decode delete request", zap.Error(err))
		http.Error(rw, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if len(deleteReq) == 0 {
		http.Error(rw, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	h.logger.Info("Delete request received",
		zap.String("userID", userID),
		zap.Int("count", len(deleteReq)))

	if err := h.service.DeleteUserURLs(userID, deleteReq); err != nil {
		h.logger.Error("Failed to queue delete task",
			zap.String("userID", userID),
			zap.Error(err))

		http.Error(rw, "Service unavailable, try again later", http.StatusServiceUnavailable)
		return
	}

	rw.WriteHeader(http.StatusAccepted)
}
