package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/mmeshcher/url-shortener/internal/models"
	"github.com/mmeshcher/url-shortener/internal/service"
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

	shortURL, err := h.service.CreateShortURL(r.Context(), req.URL)

	if err != nil {
		if errors.Is(err, service.ErrURLAlreadyExists) {
			resp := models.ShortenResponse{
				Result: shortURL,
			}

			rw.Header().Set("Content-Type", "application/json")
			rw.WriteHeader(http.StatusConflict)

			encoder := json.NewEncoder(rw)
			if err := encoder.Encode(resp); err != nil {
				h.logger.Error("Failed to encode conflict response", zap.Error(err))
				http.Error(rw, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
			return
		}

		h.logger.Error("Failed to create short URL", zap.Error(err))
		http.Error(rw, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if shortURL == "" {
		h.logger.Error("Failed to create short URL (empty result returned)")
		http.Error(rw, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	resp := models.ShortenResponse{
		Result: shortURL,
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
