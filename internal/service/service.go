package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/mmeshcher/url-shortener/internal/models"
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

type DeleteTask struct {
	UserID   string
	ShortIDs []string
}

type ShortenerService struct {
	mu          sync.RWMutex
	saveMu      sync.Mutex
	data        map[string]string
	reverseData map[string]string
	userData    map[string][]string
	deletedURLs map[string]bool
	baseURL     string
	storagePath string
	logger      *zap.Logger
	pgRepo      *repository.PostgresRepository
	useDB       bool

	deleteTasks  chan DeleteTask
	batchTimeout time.Duration
	batchSize    int
	workers      int
	wg           sync.WaitGroup
	shutdownChan chan struct{}
}

func NewShortenerService(baseURL, storagePath string, logger *zap.Logger, databaseDSN string) *ShortenerService {
	service := &ShortenerService{
		data:         make(map[string]string),
		reverseData:  make(map[string]string),
		userData:     make(map[string][]string),
		deletedURLs:  make(map[string]bool),
		baseURL:      baseURL,
		storagePath:  storagePath,
		logger:       logger,
		useDB:        databaseDSN != "",
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

	if service.useDB {
		pgRepo, err := repository.NewPostgresRepository(databaseDSN, baseURL)
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

func (s *ShortenerService) CreateShortURL(ctx context.Context, originalURL, userID string) (string, error) {
	if originalURL == "" {
		s.logger.Warn("Attempt to create short URL for empty string")
		return "", ErrEmptyURL
	}

	if _, err := url.ParseRequestURI(originalURL); err != nil {
		s.logger.Warn("Invalid URL provided", zap.String("url", originalURL), zap.Error(err))
		return "", ErrInvalidURL
	}

	if s.useDB && s.pgRepo != nil {
		shortID := s.GenerateShortID()

		savedShortID, hasConflict, err := s.pgRepo.SaveURL(ctx, shortID, originalURL, userID)

		if err != nil {
			s.logger.Error("Failed to save URL to database", zap.Error(err))
			return "", err
		}

		if hasConflict {
			fullURL, _ := url.JoinPath(s.baseURL, savedShortID)
			return fullURL, ErrURLAlreadyExists
		}

		fullURL, _ := url.JoinPath(s.baseURL, shortID)
		return fullURL, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if shortID, exists := s.reverseData[originalURL]; exists {
		fullURL, _ := url.JoinPath(s.baseURL, shortID)
		return fullURL, ErrURLAlreadyExists
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
		return "", ErrGenerateID
	}

	s.data[shortID] = originalURL
	s.reverseData[originalURL] = shortID

	if userID != "" {
		s.userData[userID] = append(s.userData[userID], shortID)
	}

	go func() {
		s.saveToFile()
	}()

	fullURL, _ := url.JoinPath(s.baseURL, shortID)
	return fullURL, nil
}

func (s *ShortenerService) GetUserURLs(ctx context.Context, userID string) ([]models.UserURL, error) {
	if s.useDB && s.pgRepo != nil {
		return s.pgRepo.GetUserURLs(ctx, userID)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if shortIDs, exists := s.userData[userID]; exists {
		var userURLs []models.UserURL
		for _, shortID := range shortIDs {
			if originalURL, ok := s.data[shortID]; ok {
				shortURL, _ := url.JoinPath(s.baseURL, shortID)
				userURLs = append(userURLs, models.UserURL{
					ShortURL:    shortURL,
					OriginalURL: originalURL,
				})
			}
		}
		return userURLs, nil
	}

	return []models.UserURL{}, nil
}

func (s *ShortenerService) GetOriginalURL(shortID string) (string, bool, bool) {
	if s.useDB && s.pgRepo != nil {
		ctx := context.Background()
		originalURL, deleted, err := s.pgRepo.GetOriginalURL(ctx, shortID)
		if err != nil {
			return "", false, false
		}
		if deleted {
			return "", true, true
		}
		return originalURL, true, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.deletedURLs[shortID] {
		return "", true, true
	}

	originalURL, exists := s.data[shortID]
	return originalURL, exists, false
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
	userData := make(map[string][]string, len(s.userData))

	for k, v := range s.data {
		data[k] = v
	}
	for k, v := range s.userData {
		userData[k] = append([]string{}, v...)
	}
	s.mu.RUnlock()

	if len(data) == 0 {
		return
	}

	type URLRecordWithUser struct {
		UUID        string `json:"uuid"`
		ShortURL    string `json:"short_url"`
		OriginalURL string `json:"original_url"`
		UserID      string `json:"user_id,omitempty"`
	}

	records := make([]URLRecordWithUser, 0, len(data))

	shortIDToUserID := make(map[string]string)
	for userID, shortIDs := range userData {
		for _, shortID := range shortIDs {
			shortIDToUserID[shortID] = userID
		}
	}

	for shortID, originalURL := range data {
		userID := shortIDToUserID[shortID]
		records = append(records, URLRecordWithUser{
			UUID:        uuid.New().String(),
			ShortURL:    shortID,
			OriginalURL: originalURL,
			UserID:      userID,
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

	type URLRecordWithUser struct {
		UUID        string `json:"uuid"`
		ShortURL    string `json:"short_url"`
		OriginalURL string `json:"original_url"`
		UserID      string `json:"user_id,omitempty"`
	}

	var records []URLRecordWithUser
	if err := json.Unmarshal(data, &records); err != nil {
		s.logger.Error("Failed to parse storage file", zap.Error(err))
		return
	}

	s.mu.Lock()
	for _, record := range records {
		s.data[record.ShortURL] = record.OriginalURL
		s.reverseData[record.OriginalURL] = record.ShortURL

		if record.UserID != "" {
			s.userData[record.UserID] = append(s.userData[record.UserID], record.ShortURL)
		}
	}
	s.mu.Unlock()
}

func (s *ShortenerService) CreateShortURLBatch(ctx context.Context, batch []models.BatchRequest, userID string) ([]models.BatchResponse, error) {
	if len(batch) == 0 {
		return nil, ErrEmptyBatch
	}

	response := make([]models.BatchResponse, 0, len(batch))

	for _, item := range batch {
		shortURL, err := s.CreateShortURL(ctx, item.OriginalURL, userID)

		if err != nil && !errors.Is(err, ErrURLAlreadyExists) {
			continue
		}

		response = append(response, models.BatchResponse{
			CorrelationID: item.CorrelationID,
			ShortURL:      shortURL,
		})
	}

	if len(response) == 0 {
		return nil, ErrNoValidURLs
	}

	return response, nil
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

	if s.useDB && s.pgRepo != nil {
		for userID, shortIDs := range userToShortIDs {
			if err := s.pgRepo.DeleteUserURLs(ctx, userID, shortIDs); err != nil {
				s.logger.Error("Failed to delete URLs in database batch",
					zap.String("userID", userID),
					zap.Int("count", len(shortIDs)),
					zap.Error(err))
			} else {
				s.logger.Info("URLs deleted in database batch",
					zap.String("userID", userID),
					zap.Int("count", len(shortIDs)))
			}
		}
	} else {
		s.mu.Lock()
		for userID, shortIDs := range userToShortIDs {
			userShortIDs, exists := s.userData[userID]
			if !exists {
				continue
			}

			userShortIDSet := make(map[string]bool)
			for _, id := range userShortIDs {
				userShortIDSet[id] = true
			}

			for _, shortID := range shortIDs {
				if userShortIDSet[shortID] {
					s.deletedURLs[shortID] = true
				}
			}
		}
		s.mu.Unlock()

		s.logger.Info("URLs marked as deleted in memory batch",
			zap.Int("batchSize", len(batch)))
	}
}

func (s *ShortenerService) Close() {
	close(s.shutdownChan)
	close(s.deleteTasks)
	s.wg.Wait()
	s.logger.Info("All delete workers stopped")
}

func (s *ShortenerService) processDeleteTask(task DeleteTask) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if s.useDB && s.pgRepo != nil {
		err := s.pgRepo.DeleteUserURLs(ctx, task.UserID, task.ShortIDs)
		if err != nil {
			s.logger.Error("Failed to delete URLs in database",
				zap.String("userID", task.UserID),
				zap.Error(err))
		} else {
			s.logger.Info("URLs deleted in database",
				zap.String("userID", task.UserID),
				zap.Int("count", len(task.ShortIDs)))
		}
	} else {
		s.mu.Lock()
		for _, shortID := range task.ShortIDs {
			if userShortIDs, exists := s.userData[task.UserID]; exists {
				for _, userShortID := range userShortIDs {
					if userShortID == shortID {
						s.deletedURLs[shortID] = true
						break
					}
				}
			}
		}
		s.mu.Unlock()

		s.logger.Info("URLs marked as deleted in memory",
			zap.String("userID", task.UserID),
			zap.Int("count", len(task.ShortIDs)))
	}
}

func (s *ShortenerService) GetURLsByShortIDs(ctx context.Context, shortIDs []string) (map[string]models.Storage, error) {
	if s.useDB && s.pgRepo != nil {
		return s.pgRepo.GetURLsByShortIDs(ctx, shortIDs)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]models.Storage)
	for _, shortID := range shortIDs {
		if originalURL, exists := s.data[shortID]; exists {
			var userID string
			for uid, shortIDs := range s.userData {
				for _, sid := range shortIDs {
					if sid == shortID {
						userID = uid
						break
					}
				}
				if userID != "" {
					break
				}
			}

			result[shortID] = models.Storage{
				ShortURL:    shortID,
				OriginalURL: originalURL,
				UserID:      userID,
				IsDeleted:   s.deletedURLs[shortID],
			}
		}
	}

	return result, nil
}
