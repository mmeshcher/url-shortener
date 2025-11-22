package service

import (
	"math/rand"
)

type ShortenerService struct {
	data map[string]string
}

func NewShortenerService() *ShortenerService {
	return &ShortenerService{
		data: make(map[string]string),
	}
}

func (s *ShortenerService) generateShortId() string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const length = 8

	result := make([]byte, length)

	for i := range result {
		result[i] = chars[rand.Intn(len(chars))]
	}

	return string(result)
}

func (s *ShortenerService) CreateShortUrl(originalUrl string) string {
	var shortId string
	for {
		shortId = s.generateShortId()
		if _, exists := s.data[shortId]; !exists {
			break
		}
	}

	s.data[shortId] = originalUrl
	return "http://localhost:8080/" + shortId
}

func (s *ShortenerService) GetOriginalUrl(shortId string) (string, bool) {
	originalUrl, exists := s.data[shortId]
	return originalUrl, exists
}
