package handler

import (
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

func TestShortenHandler(t *testing.T) {
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
		body        string
		checkBody   bool
	}

	tests := []struct {
		name    string
		request string
		body    string
		method  string
		userID  string
		want    want
		setup   func(*service.ShortenerService)
	}{
		{
			name:    "positive test",
			request: "/",
			body:    "https://practicum.yandex.ru/",
			method:  http.MethodPost,
			userID:  "test-user-1",
			want: want{
				statusCode:  201,
				contentType: "text/plain",
				body:        "http://localhost:8080/",
				checkBody:   false,
			},
		},
		{
			name:    "negative: duplicate URL conflict",
			request: "/",
			body:    "https://duplicate.yandex.ru",
			method:  http.MethodPost,
			userID:  "test-user-2",
			want: want{
				statusCode:  409,
				contentType: "text/plain",
				body:        "http://localhost:8080/",
				checkBody:   false,
			},
		},
		{
			name:    "negative: empty body",
			request: "/",
			body:    "",
			method:  http.MethodPost,
			userID:  "test-user-3",
			want: want{
				statusCode:  400,
				contentType: "text/plain; charset=utf-8",
				body:        "Empty body\n",
				checkBody:   true,
			},
		},
		{
			name:    "negative: wrong method",
			request: "/",
			body:    "https://practicum.yandex.ru/",
			method:  http.MethodGet,
			userID:  "test-user-4",
			want: want{
				statusCode:  405,
				contentType: "text/plain; charset=utf-8",
				body:        "Method Not Allowed\n",
				checkBody:   true,
			},
		},
		{
			name:    "negative: wrong path",
			request: "/api",
			body:    "https://practicum.yandex.ru/",
			method:  http.MethodGet,
			userID:  "test-user-5",
			want: want{
				statusCode:  404,
				contentType: "text/plain; charset=utf-8",
				body:        "Not Found\n",
				checkBody:   true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := service.NewShortenerService("http://localhost:8080", "", logger, "")
			h := NewHandler(service, logger, authMiddleware)
			router := h.SetupRouter()

			testCookie := createTestCookie(tt.userID)

			if tt.name == "negative: duplicate URL conflict" {
				firstReq := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body))
				firstReq.Header.Set("Content-Type", "text/plain")
				firstReq.AddCookie(testCookie)
				firstW := httptest.NewRecorder()
				router.ServeHTTP(firstW, firstReq)

				assert.Equal(t, http.StatusCreated, firstW.Code)
			}

			req := httptest.NewRequest(tt.method, tt.request, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "text/plain")
			req.AddCookie(testCookie)

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

			bodyResult, err := io.ReadAll(result.Body)
			require.NoError(t, err)
			bodyStr := string(bodyResult)

			if tt.want.checkBody {
				assert.Equal(t, tt.want.body, bodyStr,
					"Expected body '%s', got '%s' for test: %s",
					tt.want.body, bodyStr, tt.name)
			} else if tt.want.statusCode == 201 || tt.want.statusCode == 409 {
				assert.Contains(t, bodyStr, tt.want.body,
					"Expected body to contain '%s', got '%s' for test: %s",
					tt.want.body, bodyStr, tt.name)
				if tt.want.statusCode == 201 {
					assert.Greater(t, len(bodyStr), len(tt.want.body),
						"Expected body length > %d, got %d for test: %s",
						len(tt.want.body), len(bodyStr), tt.name)
				}
			}
		})
	}
}
