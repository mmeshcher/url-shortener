package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"io"

	"github.com/mmeshcher/url-shortener/internal/middleware"
	"github.com/mmeshcher/url-shortener/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestRedirectHandler(t *testing.T) {
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
		location    string
		body        string
		checkBody   bool
	}

	tests := []struct {
		name   string
		method string
		setup  func() (*service.ShortenerService, string, *http.Cookie)
		want   want
	}{
		{
			name:   "positive test",
			method: http.MethodGet,
			setup: func() (*service.ShortenerService, string, *http.Cookie) {
				service := service.NewShortenerService("http://localhost:8080", "", logger, "")
				testUserID := "test-user-123"
				testCookie := createTestCookie(testUserID)

				shortURL, _ := service.CreateShortURL(context.Background(), "https://practicum.yandex.ru/", testUserID)
				shortID := shortURL[len("http://localhost:8080/"):]
				return service, shortID, testCookie
			},
			want: want{
				statusCode:  307,
				contentType: "",
				location:    "https://practicum.yandex.ru/",
				body:        "",
				checkBody:   false,
			},
		},
		{
			name:   "negative: non-existent short URL",
			method: http.MethodGet,
			setup: func() (*service.ShortenerService, string, *http.Cookie) {
				service := service.NewShortenerService("http://localhost:8080", "", logger, "")
				testCookie := createTestCookie("test-user-456")
				return service, "nonexistent123", testCookie
			},
			want: want{
				statusCode:  404,
				contentType: "text/plain; charset=utf-8",
				location:    "",
				body:        "Not Found\n",
				checkBody:   true,
			},
		},
		{
			name:   "negative: wrong method POST",
			method: http.MethodPost,
			setup: func() (*service.ShortenerService, string, *http.Cookie) {
				service := service.NewShortenerService("http://localhost:8080", "", logger, "")
				testUserID := "test-user-789"
				testCookie := createTestCookie(testUserID)

				shortURL, _ := service.CreateShortURL(context.Background(), "https://practicum.yandex.ru/", testUserID)
				shortID := shortURL[len("http://localhost:8080/"):]
				return service, shortID, testCookie
			},
			want: want{
				statusCode:  405,
				contentType: "text/plain; charset=utf-8",
				location:    "",
				body:        "Method Not Allowed\n",
				checkBody:   true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, shortID, testCookie := tt.setup()

			request := httptest.NewRequest(tt.method, "/"+shortID, nil)
			request.AddCookie(testCookie)

			w := httptest.NewRecorder()

			h := NewHandler(service, logger, authMiddleware)
			r := h.SetupRouter()

			r.ServeHTTP(w, request)

			result := w.Result()
			defer result.Body.Close()

			assert.Equal(t, tt.want.statusCode, result.StatusCode)

			if tt.want.contentType != "" {
				assert.Equal(t, tt.want.contentType, result.Header.Get("Content-Type"))
			}

			if tt.want.location != "" {
				assert.Equal(t, tt.want.location, result.Header.Get("Location"))
			}

			bodyResult, err := io.ReadAll(result.Body)
			require.NoError(t, err)

			bodyStr := string(bodyResult)

			if tt.want.checkBody {
				assert.Equal(t, tt.want.body, bodyStr)
			} else {
				assert.Equal(t, tt.want.body, bodyStr)
			}
		})
	}
}
