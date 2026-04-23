package repository

import (
	"context"
	"github.com/mmeshcher/url-shortener/internal/models/domain"
)

// URLRepository defines the interface for URL storage operations.
type URLRepository interface {
	SaveURL(ctx context.Context, shortID, originalURL, userID string) (string, bool, error)
	GetUserURLs(ctx context.Context, userID string) ([]domain.UserURL, error)
	GetOriginalURL(ctx context.Context, shortID string) (string, bool, error)
	GetURLsByShortIDs(ctx context.Context, shortIDs []string) (map[string]domain.Storage, error)
	ProcessURLBatch(ctx context.Context, batch []domain.BatchItem) (map[string]string, error)
	DeleteUserURLs(ctx context.Context, userID string, shortIDs []string) error
	Ping(ctx context.Context) error
	Close() error
}
