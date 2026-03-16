package handler

import (
	"github.com/mmeshcher/url-shortener/internal/audit"
	"github.com/mmeshcher/url-shortener/internal/middleware"
	"github.com/mmeshcher/url-shortener/internal/service"
	"go.uber.org/zap"
)

type Handler struct {
	service        *service.ShortenerService
	logger         *zap.Logger
	authMiddleware *middleware.AuthMiddleware
	auditor        *audit.Auditor
}

func NewHandler(service *service.ShortenerService, logger *zap.Logger, authMiddleware *middleware.AuthMiddleware, auditor *audit.Auditor) *Handler {
	return &Handler{
		service:        service,
		logger:         logger,
		authMiddleware: authMiddleware,
		auditor:        auditor,
	}
}
