package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sync"

	"github.com/google/uuid"
	"github.com/mmeshcher/url-shortener/internal/models"
	"github.com/mmeshcher/url-shortener/internal/repository"
	"go.uber.org/zap"
)

type ShortenerService struct {
	mu          sync.RWMutex
	saveMu      sync.Mutex
	data        map[string]string
	reverseData map[string]string
	baseURL     string
	storagePath string
	logger      *zap.Logger
	pgRepo      *repository.PostgresRepository
	useDB       bool
}

func NewShortenerService(baseURL, storagePath string, logger *zap.Logger, databaseDSN string) *ShortenerService {
	service := &ShortenerService{
		data:        make(map[string]string),
		reverseData: make(map[string]string),
		baseURL:     baseURL,
		storagePath: storagePath,
		logger:      logger,
		useDB:       databaseDSN != "",
	}

	if service.useDB {
		pgRepo, err := repository.NewPostgresRepository(databaseDSN)
		if err != nil {
			logger.Error("Failed to connect to PostgreSQL, using file storage", zap.Error(err))
			service.useDB = false
		} else {
			service.pgRepo = pgRepo
			logger.Info("Using PostgreSQL repository")
			return service
		}
	}

	if storagePath != "" {
		service.loadFromFile()
	}

	return service
}

func (s *ShortenerService) GenerateShortID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return base64.URLEncoding.EncodeToString(bytes)[:8]
}

func (s *ShortenerService) CreateShortURL(originalURL string) string {
	if originalURL == "" {
		s.logger.Warn("Attempt to create short URL for empty string")
		return ""
	}

	if _, err := url.ParseRequestURI(originalURL); err != nil {
		s.logger.Warn("Invalid URL provided", zap.String("url", originalURL), zap.Error(err))
		return ""
	}

	if s.useDB && s.pgRepo != nil {
		ctx := context.Background()
		if existingID, err := s.pgRepo.GetShortID(ctx, originalURL); err == nil {
			fullURL, _ := url.JoinPath(s.baseURL, existingID)
			return fullURL
		}

		shortID := s.GenerateShortID()

		if err := s.pgRepo.SaveURL(ctx, shortID, originalURL); err != nil {
			s.logger.Error("Failed to save URL to database", zap.Error(err))
			return ""
		}

		fullURL, _ := url.JoinPath(s.baseURL, shortID)
		return fullURL
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if shortID, exists := s.reverseData[originalURL]; exists {
		fullURL, _ := url.JoinPath(s.baseURL, shortID)
		return fullURL
	}

	const maxAttempts = 10
	var shortID string
	var attempts int

	for attempts = 0; attempts < maxAttempts; attempts++ {
		shortID = s.GenerateShortID()
		if _, exists := s.data[shortID]; !exists {
			break
		}
	}

	if attempts == maxAttempts {
		s.logger.Error("Failed to generate unique short ID after max attempts")
		return ""
	}

	s.data[shortID] = originalURL
	s.reverseData[originalURL] = shortID

	go func() {
		s.saveToFile()
	}()

	fullURL, _ := url.JoinPath(s.baseURL, shortID)
	return fullURL
}

func (s *ShortenerService) GetOriginalURL(shortID string) (string, bool) {
	if s.useDB && s.pgRepo != nil {
		ctx := context.Background()
		originalURL, err := s.pgRepo.GetOriginalURL(ctx, shortID)
		if err != nil {
			return "", false
		}
		return originalURL, true
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	originalURL, exists := s.data[shortID]
	return originalURL, exists
}

func (s *ShortenerService) Ping() error {
	if s.useDB && s.pgRepo != nil {
		ctx := context.Background()
		return s.pgRepo.Ping(ctx)
	}

	return nil
}

func (s *ShortenerService) saveToFile() {
	if s.storagePath == "" {
		return
	}

	s.saveMu.Lock()
	defer s.saveMu.Unlock()

	s.mu.RLock()
	data := make(map[string]string, len(s.data))
	for k, v := range s.data {
		data[k] = v
	}
	s.mu.RUnlock()

	if len(data) == 0 {
		return
	}

	records := make([]models.URLRecord, 0, len(data))
	for shortID, originalURL := range data {
		records = append(records, models.URLRecord{
			UUID:        uuid.New().String(),
			ShortURL:    shortID,
			OriginalURL: originalURL,
		})
	}

	jsonData, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		s.logger.Error("Failed to marshal data for saving", zap.Error(err))
		return
	}

	os.WriteFile(s.storagePath, jsonData, 0644)
}

func (s *ShortenerService) loadFromFile() {
	if s.storagePath == "" {
		return
	}

	file, err := os.Open(s.storagePath)
	if err != nil {
		return
	}
	defer file.Close()

	data, err := os.ReadFile(s.storagePath)
	if err != nil {
		s.logger.Error("Failed to read storage file", zap.Error(err))
		return
	}

	var records []models.URLRecord
	if json.Unmarshal(data, &records) != nil {
		s.logger.Error("Failed to parse storage file", zap.Error(err))
		return
	}

	s.mu.Lock()
	for _, record := range records {
		s.data[record.ShortURL] = record.OriginalURL
		s.reverseData[record.OriginalURL] = record.ShortURL
	}
	s.mu.Unlock()
}

func (s *ShortenerService) CreateShortURLBatch(batch []models.BatchRequest) ([]models.BatchResponse, error) {
	if len(batch) == 0 {
		return nil, fmt.Errorf("empty batch")
	}

	validURLs := make([]models.BatchRequest, 0, len(batch))
	for _, item := range batch {
		if item.OriginalURL == "" {
			s.logger.Warn("Empty URL in batch", zap.String("correlation_id", item.CorrelationID))
			continue
		}

		if _, err := url.ParseRequestURI(item.OriginalURL); err != nil {
			s.logger.Warn("Invalid URL in batch",
				zap.String("correlation_id", item.CorrelationID),
				zap.String("url", item.OriginalURL),
				zap.Error(err))
			continue
		}

		validURLs = append(validURLs, item)
	}

	if len(validURLs) == 0 {
		return nil, fmt.Errorf("no valid URLs in batch")
	}

	if s.useDB && s.pgRepo != nil {
		return s.createBatchWithPostgres(validURLs)
	}

	return s.createBatchWithMemory(validURLs)
}

func (s *ShortenerService) createBatchWithPostgres(batch []models.BatchRequest) ([]models.BatchResponse, error) {
	ctx := context.Background()

	repoBatch := make([]repository.BatchItem, len(batch))
	for i, item := range batch {
		shortID := s.GenerateShortID()
		repoBatch[i] = repository.BatchItem{
			ShortID:     shortID,
			OriginalURL: item.OriginalURL,
		}
	}

	result, err := s.pgRepo.ProcessURLBatch(ctx, repoBatch)
	if err != nil {
		s.logger.Error("Failed to process URL batch", zap.Error(err))
		return nil, err
	}

	response := make([]models.BatchResponse, len(batch))
	for i, item := range batch {
		shortID := result[item.OriginalURL]
		shortURL, _ := url.JoinPath(s.baseURL, shortID)
		response[i] = models.BatchResponse{
			CorrelationID: item.CorrelationID,
			ShortURL:      shortURL,
		}
	}

	return response, nil
}

func (s *ShortenerService) createBatchWithMemory(batch []models.BatchRequest) ([]models.BatchResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	response := make([]models.BatchResponse, 0, len(batch))
	urlsToSave := make(map[string]models.BatchRequest)

	for _, item := range batch {
		if shortID, exists := s.reverseData[item.OriginalURL]; exists {
			shortURL, _ := url.JoinPath(s.baseURL, shortID)
			response = append(response, models.BatchResponse{
				CorrelationID: item.CorrelationID,
				ShortURL:      shortURL,
			})
		} else {
			shortID := s.GenerateShortID()
			urlsToSave[shortID] = item
			shortURL, _ := url.JoinPath(s.baseURL, shortID)
			response = append(response, models.BatchResponse{
				CorrelationID: item.CorrelationID,
				ShortURL:      shortURL,
			})
		}
	}

	for shortID, item := range urlsToSave {
		s.data[shortID] = item.OriginalURL
		s.reverseData[item.OriginalURL] = shortID
	}

	if len(urlsToSave) > 0 && s.storagePath != "" {
		go func() {
			s.saveToFile()
		}()
	}

	return response, nil
}
