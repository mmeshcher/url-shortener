package handler

import (
	"github.com/mmeshcher/url-shortener/internal/middleware"
	"github.com/mmeshcher/url-shortener/internal/service"
	"go.uber.org/zap"
)

type Handler struct {
	service        *service.ShortenerService
	logger         *zap.Logger
	authMiddleware *middleware.AuthMiddleware
}

func NewHandler(service *service.ShortenerService, logger *zap.Logger, authMiddleware *middleware.AuthMiddleware) *Handler {
	return &Handler{
		service:        service,
		logger:         logger,
		authMiddleware: authMiddleware,
	}
}
