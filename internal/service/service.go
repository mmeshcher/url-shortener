package service

import (
	"math/rand"
)

type ShortenerService struct {
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
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const length = 8

	result := make([]byte, length)

	for i := range result {
		result[i] = chars[rand.Intn(len(chars))]
	}

	return string(result)
}

func (s *ShortenerService) CreateShortURL(originalURL string) string {
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
	originalURL, exists := s.data[shortID]
	return originalURL, exists
}
