package service

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/mmeshcher/url-shortener/internal/models"
)

type ShortenerService struct {
	mu          sync.RWMutex
	data        map[string]string
	reverseData map[string]string
	baseURL     string
	storagePath string
}

func NewShortenerService(baseURL, storagePath string) *ShortenerService {
	service := &ShortenerService{
		data:        make(map[string]string),
		reverseData: make(map[string]string),
		baseURL:     baseURL,
		storagePath: storagePath,
	}

	service.loadFromFile()
	return service
}

func (s *ShortenerService) GenerateShortID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return base64.URLEncoding.EncodeToString(bytes)
}

func (s *ShortenerService) CreateShortURL(originalURL string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if shortID, exists := s.reverseData[originalURL]; exists {
		return s.baseURL + "/" + shortID
	}

	var shortID string
	for {
		shortID = s.GenerateShortID()
		if _, exists := s.data[shortID]; !exists {
			break
		}
	}

	s.data[shortID] = originalURL
	s.reverseData[originalURL] = shortID

	go func() {
		s.saveToFile()
	}()

	return s.baseURL + "/" + shortID
}

func (s *ShortenerService) GetOriginalURL(shortID string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	originalURL, exists := s.data[shortID]
	return originalURL, exists
}

func (s *ShortenerService) saveToFile() {
	if s.storagePath == "" {
		return
	}

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
			UUID:        generateUUID(),
			ShortURL:    shortID,
			OriginalURL: originalURL,
		})
	}

	jsonData, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
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
		return
	}

	var records []models.URLRecord
	if json.Unmarshal(data, &records) != nil {
		return
	}

	s.mu.Lock()
	for _, record := range records {
		s.data[record.ShortURL] = record.OriginalURL
		s.reverseData[record.OriginalURL] = record.ShortURL
	}
	s.mu.Unlock()
}

func generateUUID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)

	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x",
		bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:])
}
