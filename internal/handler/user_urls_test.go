package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mmeshcher/url-shortener/internal/middleware"
	"github.com/mmeshcher/url-shortener/internal/models"
	"github.com/mmeshcher/url-shortener/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestGetUserURLsHandler(t *testing.T) {
	logger := zap.NewNop()
	authMiddleware := middleware.NewAuthMiddleware("test-secret-key", logger)

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

	type want struct {
		statusCode  int
		contentType string
		urlsCount   int
		checkURLs   bool
	}

	tests := []struct {
		name        string
		method      string
		path        string
		userID      string
		cookieValue string
		setupURL    []string
		want        want
	}{
		{
			name:     "positive: user has URLs",
			method:   http.MethodGet,
			path:     "/api/user/urls",
			userID:   "test-user-1",
			setupURL: []string{"https://yandex.ru", "https://google.com", "https://github.com"},
			want: want{
				statusCode:  200,
				contentType: "application/json",
				urlsCount:   3,
				checkURLs:   true,
			},
		},
		{
			name:     "positive: user has no URLs",
			method:   http.MethodGet,
			path:     "/api/user/urls",
			userID:   "test-user-2",
			setupURL: []string{},
			want: want{
				statusCode:  204,
				contentType: "",
				urlsCount:   0,
				checkURLs:   false,
			},
		},
		{
			name:     "positive: user has only one URL",
			method:   http.MethodGet,
			path:     "/api/user/urls",
			userID:   "test-user-3",
			setupURL: []string{"https://practicum.yandex.ru"},
			want: want{
				statusCode:  200,
				contentType: "application/json",
				urlsCount:   1,
				checkURLs:   true,
			},
		},
		{
			name:     "positive: unauthorized (no cookie)",
			method:   http.MethodGet,
			path:     "/api/user/urls",
			userID:   "",
			setupURL: []string{},
			want: want{
				statusCode:  204,
				contentType: "",
				urlsCount:   0,
				checkURLs:   false,
			},
		},
		{
			name:     "negative: wrong method POST",
			method:   http.MethodPost,
			path:     "/api/user/urls",
			userID:   "test-user-4",
			setupURL: []string{},
			want: want{
				statusCode:  405,
				contentType: "text/plain; charset=utf-8",
				urlsCount:   0,
				checkURLs:   false,
			},
		},
		{
			name:        "negative: invalid cookie signature",
			method:      http.MethodGet,
			path:        "/api/user/urls",
			cookieValue: "test-user-invalid.wrong-signature-here",
			setupURL:    []string{},
			want: want{
				statusCode:  401,
				contentType: "text/plain; charset=utf-8",
				urlsCount:   0,
				checkURLs:   false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := service.NewShortenerService("http://localhost:8080", "", logger, "")
			h := NewHandler(service, logger, authMiddleware)
			router := h.SetupRouter()

			if tt.userID != "" && len(tt.setupURL) > 0 {
				testCookie := createTestCookie(tt.userID)

				for _, url := range tt.setupURL {
					req := httptest.NewRequest(http.MethodPost, "/api/shorten",
						strings.NewReader(`{"url":"`+url+`"}`))
					req.Header.Set("Content-Type", "application/json")
					req.AddCookie(testCookie)

					w := httptest.NewRecorder()
					router.ServeHTTP(w, req)

					assert.Equal(t, http.StatusCreated, w.Code,
						"Failed to create URL for setup: %s", url)
				}
			}

			req := httptest.NewRequest(tt.method, tt.path, nil)

			if tt.userID != "" {
				testCookie := createTestCookie(tt.userID)
				req.AddCookie(testCookie)
			} else if tt.cookieValue != "" {
				req.AddCookie(&http.Cookie{
					Name:  "user_id",
					Value: tt.cookieValue,
				})
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			result := w.Result()
			defer result.Body.Close()

			assert.Equal(t, tt.want.statusCode, result.StatusCode,
				"Expected status %d, got %d for test: %s",
				tt.want.statusCode, result.StatusCode, tt.name)

			if tt.want.contentType != "" {
				contentType := result.Header.Get("Content-Type")
				assert.Equal(t, tt.want.contentType, contentType,
					"Expected content-type %s, got %s for test: %s",
					tt.want.contentType, contentType, tt.name)
			}

			switch tt.want.statusCode {
			case http.StatusOK:
				var userURLs []models.UserURL
				err := json.NewDecoder(result.Body).Decode(&userURLs)
				require.NoError(t, err, "Failed to decode JSON response")

				assert.Len(t, userURLs, tt.want.urlsCount,
					"Expected %d URLs, got %d for test: %s",
					tt.want.urlsCount, len(userURLs), tt.name)

				if tt.want.checkURLs {
					for _, url := range userURLs {
						assert.NotEmpty(t, url.ShortURL,
							"ShortURL should not be empty")
						assert.NotEmpty(t, url.OriginalURL,
							"OriginalURL should not be empty")
						assert.Contains(t, url.ShortURL, "http://localhost:8080/",
							"ShortURL should contain base URL")
					}
				}

			case http.StatusNoContent:
				body, err := io.ReadAll(result.Body)
				require.NoError(t, err)
				assert.Empty(t, string(body),
					"Expected empty body for 204 status, got: %s", string(body))

			case http.StatusUnauthorized:
				body, err := io.ReadAll(result.Body)
				require.NoError(t, err)
				assert.Contains(t, string(body), "Unauthorized",
					"Expected 'Unauthorized' in response for test: %s", tt.name)
			}
		})
	}
}
