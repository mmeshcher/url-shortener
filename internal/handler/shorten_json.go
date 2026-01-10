package handler

import (
	"encoding/json"
	"net/http"

	"github.com/mmeshcher/url-shortener/internal/models"
	"go.uber.org/zap"
)

func (h *Handler) ShortenJSONHandler(rw http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		http.Error(rw, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	var req models.ShortenRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		http.Error(rw, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		http.Error(rw, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	shortURL, conflict, err := h.service.CreateShortURL(req.URL)
	if err != nil {
		if shortURL == "" {
			h.logger.Error("Failed to create short URL", zap.Error(err))
			http.Error(rw, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
	}

	resp := models.ShortenResponse{
		Result: shortURL,
	}

	rw.Header().Set("Content-Type", "application/json")

	if conflict {
		rw.WriteHeader(http.StatusConflict)
	} else {
		rw.WriteHeader(http.StatusCreated)
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusCreated)

	encoder := json.NewEncoder(rw)
	if err := encoder.Encode(resp); err != nil {
		h.logger.Error("Failed to encode response", zap.Error(err))
		http.Error(rw, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}
