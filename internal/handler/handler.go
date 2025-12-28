package handler

import (
	"github.com/mmeshcher/url-shortener/internal/service"
	"go.uber.org/zap"
)

type Handler struct {
	service *service.ShortenerService
	logger  *zap.Logger
}

func NewHandler(service *service.ShortenerService, logger *zap.Logger) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}
