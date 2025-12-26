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
		baseURL:     baseURL,
		reverseData: make(map[string]string),
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

	s.saveToFile()

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
	defer s.mu.RUnlock()

	records := make([]models.URLRecord, 0, len(s.data))

	for shortID, originalURL := range s.data {
		record := models.URLRecord{
			UUID:        generateUUID(),
			ShortURL:    shortID,
			OriginalURL: originalURL,
		}
		records = append(records, record)
	}

	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling data: %v\n", err)
		return
	}

	err = os.WriteFile(s.storagePath, data, 0644)
	if err != nil {
		fmt.Printf("Error writing to file %s: %v\n", s.storagePath, err)
	}
}

func (s *ShortenerService) loadFromFile() {
	if s.storagePath == "" {
		return
	}

	if _, err := os.Stat(s.storagePath); os.IsNotExist(err) {
		return
	}

	data, err := os.ReadFile(s.storagePath)
	if err != nil {
		fmt.Printf("Error reading file %s: %v\n", s.storagePath, err)
		return
	}

	var records []models.URLRecord
	err = json.Unmarshal(data, &records)
	if err != nil {
		fmt.Printf("Error unmarshaling data from %s: %v\n", s.storagePath, err)
		return
	}

	s.mu.Lock()
	for _, record := range records {
		s.data[record.ShortURL] = record.OriginalURL
		s.reverseData[record.OriginalURL] = record.ShortURL
	}
	s.mu.Unlock()

	fmt.Printf("Loaded %d URL records from %s\n", len(records), s.storagePath)
}

func generateUUID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)

	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x",
		bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:])
}
