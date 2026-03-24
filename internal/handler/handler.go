// Package handler contains HTTP handlers for the URL shortener service.
package handler

import (
	"github.com/mmeshcher/url-shortener/internal/audit"
	"github.com/mmeshcher/url-shortener/internal/middleware"
	"github.com/mmeshcher/url-shortener/internal/service"
	"go.uber.org/zap"
)

// Handler provides HTTP endpoints for URL shortening and management.
type Handler struct {
	service        *service.ShortenerService
	logger         *zap.Logger
	authMiddleware *middleware.AuthMiddleware
	auditor        *audit.Auditor
}

// NewHandler creates a new Handler instance with the provided dependencies.
func NewHandler(service *service.ShortenerService, logger *zap.Logger, authMiddleware *middleware.AuthMiddleware, auditor *audit.Auditor) *Handler {
	return &Handler{
		service:        service,
		logger:         logger,
		authMiddleware: authMiddleware,
		auditor:        auditor,
	}
}
