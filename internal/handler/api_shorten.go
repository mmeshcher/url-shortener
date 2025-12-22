package handler

import (
	"encoding/json"
	"net/http"

	"github.com/mmeshcher/url-shortener/internal/models"
)

func (h *Handler) APIShortenHandler(rw http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(rw, "MethodNotAllowed", http.StatusMethodNotAllowed)
		return
	}

	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		http.Error(rw, "Content-Type must be application/json", http.StatusBadRequest)
		return
	}

	var req models.ShortenRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		http.Error(rw, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		http.Error(rw, "URL cannot be empty", http.StatusBadRequest)
		return
	}

	shortURL := h.service.CreateShortURL(req.URL)

	resp := models.ShortenResponse{
		Result: shortURL,
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusCreated)

	encoder := json.NewEncoder(rw)
	if err := encoder.Encode(resp); err != nil {
		http.Error(rw, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}
