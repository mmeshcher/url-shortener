package handler

import (
	"net/http"

	"go.uber.org/zap"
)

// PingHandler handles GET requests to check the availability of the service and its storage layer.
func (h *Handler) PingHandler(rw http.ResponseWriter, r *http.Request) {
	if err := h.service.Ping(); err != nil {
		h.logger.Error("Database ping failed", zap.Error(err))
		http.Error(rw, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	rw.WriteHeader(http.StatusOK)
}
