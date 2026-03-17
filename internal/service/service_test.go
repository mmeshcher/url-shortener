package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestShortenerService(t *testing.T) {
	logger := zap.NewNop()
	s := NewShortenerService("http://localhost:8080", "", logger, "")

	t.Run("Create and Get URL", func(t *testing.T) {
		originalURL := "https://example.com"
		userID := "user1"

		shortURL, err := s.CreateShortURL(context.Background(), originalURL, userID)
		require.NoError(t, err)
		assert.NotEmpty(t, shortURL)

		// Get original URL
		// The service returns the full URL, we need to extract the shortID
		// But s.data contains shortID -> originalURL
		// Let's find the shortID from the full URL
		var shortID string
		for id, url := range s.data {
			if url == originalURL {
				shortID = id
				break
			}
		}

		gotURL, exists, deleted := s.GetOriginalURL(shortID)
		assert.True(t, exists)
		assert.False(t, deleted)
		assert.Equal(t, originalURL, gotURL)
	})

	t.Run("Invalid URL", func(t *testing.T) {
		_, err := s.CreateShortURL(context.Background(), "not-a-url", "user1")
		assert.Error(t, err)
	})

	t.Run("Empty URL", func(t *testing.T) {
		_, err := s.CreateShortURL(context.Background(), "", "user1")
		assert.Error(t, err)
	})

	t.Run("Duplicate URL", func(t *testing.T) {
		url := "https://duplicate.com"
		_, err := s.CreateShortURL(context.Background(), url, "user1")
		require.NoError(t, err)

		_, err = s.CreateShortURL(context.Background(), url, "user1")
		assert.ErrorIs(t, err, ErrURLAlreadyExists)
	})
}

func BenchmarkCreateShortURL(b *testing.B) {
	logger := zap.NewNop()
	s := NewShortenerService("http://localhost:8080", "", logger, "")
	ctx := context.Background()
	userID := "user1"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		url := "https://example.com/" + string(rune(i))
		s.CreateShortURL(ctx, url, userID)
	}
}

func BenchmarkGetOriginalURL(b *testing.B) {
	logger := zap.NewNop()
	s := NewShortenerService("http://localhost:8080", "", logger, "")
	ctx := context.Background()
	userID := "user1"
	url := "https://example.com"
	_, _ = s.CreateShortURL(ctx, url, userID)

	// Extract shortID
	var shortID string
	for id := range s.data {
		shortID = id
		break
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.GetOriginalURL(shortID)
	}
}
