package repository

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"

	"github.com/google/uuid"
	"github.com/mmeshcher/url-shortener/internal/models/domain"
	"go.uber.org/zap"
)

// MemoryRepository handles in-memory URL storage with optional file persistence.
type MemoryRepository struct {
	mu          sync.RWMutex
	data        map[string]string
	reverseData map[string]string
	userData    map[string][]string
	deletedURLs map[string]bool
	storagePath string
	logger      *zap.Logger
}

// NewMemoryRepository creates a new MemoryRepository instance.
func NewMemoryRepository(storagePath string, logger *zap.Logger) *MemoryRepository {
	repo := &MemoryRepository{
		data:        make(map[string]string),
		reverseData: make(map[string]string),
		userData:    make(map[string][]string),
		deletedURLs: make(map[string]bool),
		storagePath: storagePath,
		logger:      logger,
	}

	if storagePath != "" {
		repo.loadFromFile()
	}

	return repo
}

func (m *MemoryRepository) SaveURL(ctx context.Context, shortID, originalURL, userID string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if existingID, exists := m.reverseData[originalURL]; exists {
		return existingID, true, nil
	}

	m.data[shortID] = originalURL
	m.reverseData[originalURL] = shortID
	if userID != "" {
		m.userData[userID] = append(m.userData[userID], shortID)
	}

	return shortID, false, nil
}

func (m *MemoryRepository) GetUserURLs(ctx context.Context, userID string) ([]domain.UserURL, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	shortIDs, exists := m.userData[userID]
	if !exists {
		return nil, nil // Return nil for empty slice as per comment 8
	}

	userURLs := make([]domain.UserURL, 0, len(shortIDs))
	for _, shortID := range shortIDs {
		if originalURL, ok := m.data[shortID]; ok {
			userURLs = append(userURLs, domain.UserURL{
				ShortURL:    shortID, // Service will append baseURL
				OriginalURL: originalURL,
			})
		}
	}

	return userURLs, nil
}

func (m *MemoryRepository) GetOriginalURL(ctx context.Context, shortID string) (string, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.deletedURLs[shortID] {
		return "", true, nil
	}

	originalURL, exists := m.data[shortID]
	if !exists {
		return "", false, nil
	}

	return originalURL, false, nil
}

func (m *MemoryRepository) GetURLsByShortIDs(ctx context.Context, shortIDs []string) (map[string]domain.Storage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]domain.Storage)
	for _, shortID := range shortIDs {
		if originalURL, exists := m.data[shortID]; exists {
			var userID string
			for uid, sids := range m.userData {
				for _, sid := range sids {
					if sid == shortID {
						userID = uid
						break
					}
				}
				if userID != "" {
					break
				}
			}

			result[shortID] = domain.Storage{
				ShortURL:    shortID,
				OriginalURL: originalURL,
				UserID:      userID,
				IsDeleted:   m.deletedURLs[shortID],
			}
		}
	}

	return result, nil
}

func (m *MemoryRepository) ProcessURLBatch(ctx context.Context, batch []domain.BatchItem) (map[string]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make(map[string]string)
	for _, item := range batch {
		if existingID, exists := m.reverseData[item.OriginalURL]; exists {
			result[item.OriginalURL] = existingID
		} else {
			m.data[item.ShortID] = item.OriginalURL
			m.reverseData[item.OriginalURL] = item.ShortID
			if item.UserID != "" {
				m.userData[item.UserID] = append(m.userData[item.UserID], item.ShortID)
			}
			result[item.OriginalURL] = item.ShortID
		}
	}

	return result, nil
}

func (m *MemoryRepository) DeleteUserURLs(ctx context.Context, userID string, shortIDs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	userShortIDs, exists := m.userData[userID]
	if !exists {
		return nil
	}

	userShortIDSet := make(map[string]bool)
	for _, id := range userShortIDs {
		userShortIDSet[id] = true
	}

	for _, shortID := range shortIDs {
		if userShortIDSet[shortID] {
			m.deletedURLs[shortID] = true
		}
	}

	return nil
}

func (m *MemoryRepository) GetStats(ctx context.Context) (int, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	urlsCount := len(m.data)
	usersCount := len(m.userData)

	return urlsCount, usersCount, nil
}

func (m *MemoryRepository) Ping(ctx context.Context) error {
	return nil
}

func (m *MemoryRepository) Close() error {
	if m.storagePath != "" {
		return m.saveToFile()
	}
	return nil
}

func (m *MemoryRepository) saveToFile() error {
	if m.storagePath == "" {
		return nil
	}

	m.mu.RLock()
	data := make(map[string]string, len(m.data))
	userData := make(map[string][]string, len(m.userData))

	for k, v := range m.data {
		data[k] = v
	}
	for k, v := range m.userData {
		userData[k] = append([]string{}, v...)
	}
	m.mu.RUnlock()

	if len(data) == 0 {
		return nil
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
			UUID:        uuid.NewString(), // Fixed as per comment 3
			ShortURL:    shortID,
			OriginalURL: originalURL,
			UserID:      userID,
		})
	}

	jsonData, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		m.logger.Error("Failed to marshal data for saving", zap.Error(err))
		return err
	}

	return os.WriteFile(m.storagePath, jsonData, 0644)
}

func (m *MemoryRepository) loadFromFile() {
	if m.storagePath == "" {
		return
	}

	data, err := os.ReadFile(m.storagePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return
		}
		m.logger.Error("Failed to read storage file", zap.Error(err))
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
		m.logger.Error("Failed to parse storage file", zap.Error(err))
		return
	}

	m.mu.Lock()
	for _, record := range records {
		m.data[record.ShortURL] = record.OriginalURL
		m.reverseData[record.OriginalURL] = record.ShortURL

		if record.UserID != "" {
			m.userData[record.UserID] = append(m.userData[record.UserID], record.ShortURL)
		}
	}
	m.mu.Unlock()
}
