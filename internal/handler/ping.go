package handler

import (
	"net/http"

	"go.uber.org/zap"
)

func (h *Handler) PingHandler(rw http.ResponseWriter, r *http.Request) {
	if err := h.service.Ping(); err != nil {
		h.logger.Error("Database ping failed", zap.Error(err))
		http.Error(rw, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	rw.WriteHeader(http.StatusOK)
}
