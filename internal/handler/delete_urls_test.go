package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mmeshcher/url-shortener/internal/middleware"
	"github.com/mmeshcher/url-shortener/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestDeleteUserURLsHandler(t *testing.T) {
	logger := zap.NewNop()
	middleware.InitAuthMiddleware("test-secret-key", logger)

	createTestCookie := func(userID string) *http.Cookie {
		mac := hmac.New(sha256.New, []byte("test-secret-key"))
		mac.Write([]byte(userID))
		signature := mac.Sum(nil)
		signedValue := userID + "." + hex.EncodeToString(signature)

		return &http.Cookie{
			Name:  "user_id",
			Value: signedValue,
		}
	}

	tests := []struct {
		name           string
		method         string
		path           string
		body           string
		headers        map[string]string
		cookie         *http.Cookie
		setupFunc      func(*service.ShortenerService, string)
		expectedStatus int
	}{
		{
			name:   "positive delete request",
			method: http.MethodDelete,
			path:   "/api/user/urls",
			body:   `["short1", "short2", "short3"]`,
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			cookie: createTestCookie("test-user-1"),
			setupFunc: func(s *service.ShortenerService, userID string) {
				s.CreateShortURL(context.Background(), "https://example.com/1", userID)
				s.CreateShortURL(context.Background(), "https://example.com/2", userID)
				s.CreateShortURL(context.Background(), "https://example.com/3", userID)
			},
			expectedStatus: http.StatusAccepted,
		},
		{
			name:   "negative: wrong method POST",
			method: http.MethodPost,
			path:   "/api/user/urls",
			body:   `["short1"]`,
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			cookie:         createTestCookie("test-user-2"),
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:   "negative: wrong content type",
			method: http.MethodDelete,
			path:   "/api/user/urls",
			body:   `["short1"]`,
			headers: map[string]string{
				"Content-Type": "text/plain",
			},
			cookie:         createTestCookie("test-user-3"),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "negative: empty array",
			method: http.MethodDelete,
			path:   "/api/user/urls",
			body:   `[]`,
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			cookie:         createTestCookie("test-user-4"),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "negative: invalid JSON",
			method: http.MethodDelete,
			path:   "/api/user/urls",
			body:   `["short1",]`,
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			cookie:         createTestCookie("test-user-5"),
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "negative: invalid cookie",
			method: http.MethodDelete,
			path:   "/api/user/urls",
			body:   `["short1"]`,
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			cookie: &http.Cookie{
				Name:  "user_id",
				Value: "invalid-signature",
			},
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := service.NewShortenerService("http://localhost:8080", "", logger, "")

			if tt.setupFunc != nil && tt.cookie != nil {
				parts := strings.Split(tt.cookie.Value, ".")
				if len(parts) == 2 {
					userID := parts[0]
					tt.setupFunc(service, userID)
				}
			}

			h := NewHandler(service, logger)
			router := h.SetupRouter()

			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			if tt.cookie != nil {
				req.AddCookie(tt.cookie)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			result := w.Result()
			defer result.Body.Close()

			assert.Equal(t, tt.expectedStatus, result.StatusCode,
				"Expected status %d but got %d for test: %s",
				tt.expectedStatus, result.StatusCode, tt.name)

			if tt.expectedStatus == http.StatusAccepted {
				bodyBytes, err := io.ReadAll(result.Body)
				require.NoError(t, err)
				assert.Empty(t, string(bodyBytes))
			}
		})
	}
}
