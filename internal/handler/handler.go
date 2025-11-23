package handler

import (
	"github.com/mmeshcher/url-shortener/internal/service"
)

type Handler struct {
	service *service.ShortenerService
}

func NewHandler(service *service.ShortenerService) *Handler {
	return &Handler{
		service: service,
	}
}
