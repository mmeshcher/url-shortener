package service

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
)

type ShortenerService struct {
	mu      sync.RWMutex
	data    map[string]string
	baseURL string
}

func NewShortenerService(baseURL string) *ShortenerService {
	return &ShortenerService{
		data:    make(map[string]string),
		baseURL: baseURL,
	}
}

func (s *ShortenerService) GenerateShortID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)

	return base64.URLEncoding.EncodeToString(bytes)
}

func (s *ShortenerService) CreateShortURL(originalURL string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	var shortID string
	for {
		shortID = s.GenerateShortID()
		if _, exists := s.data[shortID]; !exists {
			break
		}
	}

	s.data[shortID] = originalURL
	return s.baseURL + "/" + shortID
}

func (s *ShortenerService) GetOriginalURL(shortID string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	originalURL, exists := s.data[shortID]
	return originalURL, exists
}
