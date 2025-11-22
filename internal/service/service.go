package service

import (
	"math/rand"
)

type ShortnerService struct {
	data map[string]string
}

func NewShortenerService() *ShortnerService {
	return &ShortnerService{
		data: make(map[string]string),
	}
}

func (s *ShortnerService) generateShortId() string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const length = 8

	result := make([]byte, length)

	for i := range result {
		result[i] = chars[rand.Intn(len(chars))]
	}

	return string(result)
}

func (s *ShortnerService) CreateShortUrl(originalUrl string) string {
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

func (s *ShortnerService) GetOriginalUrl(shortId string) (string, bool) {
	originalUrl, exists := s.data[shortId]
	return originalUrl, exists
}
