// Package service provides the business logic for URL shortening, storage, and retrieval.
package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/url"
	"sync"
	"time"

	"github.com/mmeshcher/url-shortener/internal/models/domain"
	"github.com/mmeshcher/url-shortener/internal/repository"
	"go.uber.org/zap"
)

var (
	ErrEmptyURL         = errors.New("empty url")
	ErrInvalidURL       = errors.New("invalid url")
	ErrURLAlreadyExists = errors.New("url already exists")
	ErrEmptyBatch       = errors.New("empty batch")
	ErrNoValidURLs      = errors.New("no valid urls in batch")
	ErrGenerateID       = errors.New("failed to generate unique id")
)

// DeleteTask represents a background job to mark multiple short URLs as deleted for a user.
type DeleteTask struct {
	UserID   string   // ID of the user who owns the URLs.
	ShortIDs []string // List of short URL IDs to be deleted.
}

// ShortenerService handles URL shortening logic and interacts with the storage layer.
type ShortenerService struct {
	repo         repository.URLRepository
	baseURL      string
	logger       *zap.Logger
	deleteTasks  chan DeleteTask
	batchTimeout time.Duration
	batchSize    int
	workers      int
	wg           sync.WaitGroup
	shutdownChan chan struct{}
}

// NewShortenerService creates and initializes a new ShortenerService.
func NewShortenerService(baseURL string, repo repository.URLRepository, logger *zap.Logger) *ShortenerService {
	service := &ShortenerService{
		repo:         repo,
		baseURL:      baseURL,
		logger:       logger,
		deleteTasks:  make(chan DeleteTask, 1000),
		batchTimeout: 500 * time.Millisecond,
		batchSize:    100,
		workers:      3,
		shutdownChan: make(chan struct{}),
	}

	for i := 0; i < service.workers; i++ {
		service.wg.Add(1)
		go service.deleteWorker(i)
	}

	return service
}

// GenerateShortID creates a random 8-character string for short URL identification.
func (s *ShortenerService) GenerateShortID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return base64.URLEncoding.EncodeToString(bytes)[:8]
}

// CreateShortURL shortens a single URL and associates it with a user.
// It returns the full short URL. If the URL already exists, it returns ErrURLAlreadyExists.
func (s *ShortenerService) CreateShortURL(ctx context.Context, originalURL, userID string) (string, error) {
	if originalURL == "" {
		s.logger.Warn("Attempt to create short URL for empty string")
		return "", ErrEmptyURL
	}

	if _, err := url.ParseRequestURI(originalURL); err != nil {
		s.logger.Warn("Invalid URL provided", zap.String("url", originalURL), zap.Error(err))
		return "", ErrInvalidURL
	}

	shortID := s.GenerateShortID()
	savedShortID, hasConflict, err := s.repo.SaveURL(ctx, shortID, originalURL, userID)

	if err != nil {
		s.logger.Error("Failed to save URL to repository", zap.Error(err))
		return "", err
	}

	if hasConflict {
		return s.baseURL + "/" + savedShortID, ErrURLAlreadyExists
	}

	return s.baseURL + "/" + shortID, nil
}

// GetUserURLs returns a list of all URLs created by the specified user.
func (s *ShortenerService) GetUserURLs(ctx context.Context, userID string) ([]domain.UserURL, error) {
	urls, err := s.repo.GetUserURLs(ctx, userID)
	if err != nil {
		return nil, err
	}

	if len(urls) == 0 {
		return nil, nil // Return nil for empty slice as requested in comment 8
	}

	for i := range urls {
		urls[i].ShortURL = s.baseURL + "/" + urls[i].ShortURL
	}

	return urls, nil
}

// GetOriginalURL retrieves the original URL corresponding to the given short ID.
// It returns the original URL, a boolean indicating if it exists, and a boolean indicating if it was deleted.
func (s *ShortenerService) GetOriginalURL(shortID string) (string, bool, bool) {
	ctx := context.Background()
	originalURL, deleted, err := s.repo.GetOriginalURL(ctx, shortID)
	if err != nil {
		return "", false, false
	}

	if deleted {
		return "", true, true
	}

	if originalURL == "" {
		return "", false, false
	}

	return originalURL, true, false
}

// Ping checks the availability of the storage layer (e.g., database).
func (s *ShortenerService) Ping() error {
	ctx := context.Background()
	return s.repo.Ping(ctx)
}

// CreateShortURLBatch shortens multiple URLs in a single request.
func (s *ShortenerService) CreateShortURLBatch(ctx context.Context, batch []domain.BatchRequest, userID string) ([]domain.BatchResponse, error) {
	if len(batch) == 0 {
		return nil, ErrEmptyBatch
	}

	repoBatch := make([]domain.BatchItem, 0, len(batch))
	for _, item := range batch {
		if item.OriginalURL == "" {
			continue
		}
		if _, err := url.ParseRequestURI(item.OriginalURL); err != nil {
			continue
		}

		shortID := s.GenerateShortID()
		repoBatch = append(repoBatch, domain.BatchItem{
			ShortID:     shortID,
			OriginalURL: item.OriginalURL,
			UserID:      userID,
		})
	}

	if len(repoBatch) == 0 {
		return nil, ErrNoValidURLs
	}

	result, err := s.repo.ProcessURLBatch(ctx, repoBatch)
	if err != nil {
		s.logger.Error("Failed to process URL batch", zap.Error(err))
		return nil, err
	}

	response := make([]domain.BatchResponse, 0, len(batch))
	for _, b := range batch {
		if shortID, ok := result[b.OriginalURL]; ok {
			shortURL := s.baseURL + "/" + shortID
			response = append(response, domain.BatchResponse{
				CorrelationID: b.CorrelationID,
				ShortURL:      shortURL,
			})
		}
	}

	return response, nil
}

// DeleteUserURLs queues a background task to delete the specified short URLs for a user.
func (s *ShortenerService) DeleteUserURLs(userID string, shortIDs []string) error {
	if len(shortIDs) == 0 {
		return nil
	}

	task := DeleteTask{
		UserID:   userID,
		ShortIDs: shortIDs,
	}

	select {
	case s.deleteTasks <- task:
		s.logger.Info("Delete task queued",
			zap.String("userID", userID),
			zap.Int("count", len(shortIDs)))
		return nil
	case <-time.After(5 * time.Second):
		s.logger.Error("Delete queue is full, timeout exceeded",
			zap.String("userID", userID))
		return errors.New("delete service busy, try again later")
	}
}

// deleteWorker runs as a background goroutine to process URL deletion tasks in batches.
func (s *ShortenerService) deleteWorker(id int) {
	defer s.wg.Done()

	s.logger.Debug("Delete worker started", zap.Int("workerID", id))

	batch := make([]DeleteTask, 0, s.batchSize)
	timer := time.NewTimer(s.batchTimeout)
	defer timer.Stop()

	for {
		timer.Reset(s.batchTimeout)

		select {
		case task, ok := <-s.deleteTasks:
			if !ok {
				if len(batch) > 0 {
					s.processBatch(batch)
				}
				s.logger.Debug("Delete worker stopped", zap.Int("workerID", id))
				return
			}

			batch = append(batch, task)

			if len(batch) >= s.batchSize {
				s.processBatch(batch)
				batch = batch[:0]
				if !timer.Stop() {
					<-timer.C
				}
			}

		case <-timer.C:
			if len(batch) > 0 {
				s.processBatch(batch)
				batch = batch[:0]
			}

		case <-s.shutdownChan:
			if len(batch) > 0 {
				s.processBatch(batch)
			}
			s.logger.Debug("Delete worker stopped by shutdown", zap.Int("workerID", id))
			return
		}
	}
}

// processBatch handles the deletion of a batch of tasks.
func (s *ShortenerService) processBatch(batch []DeleteTask) {
	if len(batch) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	userToShortIDs := make(map[string][]string)
	for _, task := range batch {
		userToShortIDs[task.UserID] = append(userToShortIDs[task.UserID], task.ShortIDs...)
	}

	for userID, shortIDs := range userToShortIDs {
		if err := s.repo.DeleteUserURLs(ctx, userID, shortIDs); err != nil {
			s.logger.Error("Failed to delete URLs in repository batch",
				zap.String("userID", userID),
				zap.Int("count", len(shortIDs)),
				zap.Error(err))
		} else {
			s.logger.Info("URLs deleted in repository batch",
				zap.String("userID", userID),
				zap.Int("count", len(shortIDs)))
		}
	}
}

// Close gracefully shuts down the ShortenerService, waiting for background workers to finish.
func (s *ShortenerService) Close() {
	close(s.shutdownChan)
	close(s.deleteTasks)
	s.wg.Wait()
	s.logger.Info("All delete workers stopped")

	if err := s.repo.Close(); err != nil {
		s.logger.Error("Failed to close repository", zap.Error(err))
	} else {
		s.logger.Info("Repository closed")
	}
}

// processDeleteTask marks URLs as deleted in storage.
func (s *ShortenerService) processDeleteTask(task DeleteTask) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := s.repo.DeleteUserURLs(ctx, task.UserID, task.ShortIDs)
	if err != nil {
		s.logger.Error("Failed to delete URLs in repository",
			zap.String("userID", task.UserID),
			zap.Error(err))
	} else {
		s.logger.Info("URLs deleted in repository",
			zap.String("userID", task.UserID),
			zap.Int("count", len(task.ShortIDs)))
	}
}

// GetURLsByShortIDs retrieves storage records for multiple short IDs.
func (s *ShortenerService) GetURLsByShortIDs(ctx context.Context, shortIDs []string) (map[string]domain.Storage, error) {
	return s.repo.GetURLsByShortIDs(ctx, shortIDs)
}
