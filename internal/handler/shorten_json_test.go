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

func TestShortenJSONHandler(t *testing.T) {
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
		statusCode     int
		contentType    string
		checkResult    bool
		checkError     bool
		expectConflict bool
	}

	tests := []struct {
		name    string
		method  string
		path    string
		body    string
		headers map[string]string
		userID  string
		want    want
	}{
		{
			name:   "positive test with JSON",
			method: http.MethodPost,
			path:   "/api/shorten",
			body:   `{"url":"https://practicum.yandex.ru"}`,
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			userID: "test-user-1",
			want: want{
				statusCode:  http.StatusCreated,
				contentType: "application/json",
				checkResult: true,
			},
		},
		{
			name:   "negative: duplicate URL conflict",
			method: http.MethodPost,
			path:   "/api/shorten",
			body:   `{"url":"https://duplicate.yandex.ru"}`,
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			userID: "test-user-2",
			want: want{
				statusCode:     http.StatusConflict,
				contentType:    "application/json",
				checkResult:    true,
				expectConflict: true,
			},
		},
		{
			name:   "negative: empty URL",
			method: http.MethodPost,
			path:   "/api/shorten",
			body:   `{"url":""}`,
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			userID: "test-user-3",
			want: want{
				statusCode: http.StatusBadRequest,
				checkError: true,
			},
		},
		{
			name:   "negative: invalid JSON",
			method: http.MethodPost,
			path:   "/api/shorten",
			body:   `{"url":"https://practicum.yandex.ru",}`,
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			userID: "test-user-4",
			want: want{
				statusCode: http.StatusBadRequest,
				checkError: true,
			},
		},
		{
			name:   "negative: wrong content type",
			method: http.MethodPost,
			path:   "/api/shorten",
			body:   `{"url":"https://practicum.yandex.ru"}`,
			headers: map[string]string{
				"Content-Type": "text/plain",
			},
			userID: "test-user-5",
			want: want{
				statusCode: http.StatusBadRequest,
			},
		},
		{
			name:   "negative: wrong method GET",
			method: http.MethodGet,
			path:   "/api/shorten",
			body:   `{"url":"https://practicum.yandex.ru"}`,
			headers: map[string]string{
				"Content-Type": "application/json",
			},
			userID: "test-user-6",

			want: want{
				statusCode: http.StatusMethodNotAllowed,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := service.NewShortenerService("http://localhost:8080", "", logger, "")
			h := NewHandler(service, logger, authMiddleware)
			router := h.SetupRouter()

			testCookie := createTestCookie(tt.userID)

			if tt.want.expectConflict {
				firstReq := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
				for key, value := range tt.headers {
					firstReq.Header.Set(key, value)
				}
				firstReq.AddCookie(testCookie)
				firstW := httptest.NewRecorder()
				router.ServeHTTP(firstW, firstReq)

				assert.Equal(t, http.StatusCreated, firstW.Code)
			}

			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}
			req.AddCookie(testCookie)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			result := w.Result()
			defer result.Body.Close()

			assert.Equal(t, tt.want.statusCode, result.StatusCode)

			if tt.want.contentType != "" {
				assert.Equal(t, tt.want.contentType, result.Header.Get("Content-Type"))
			}

			if tt.want.checkResult {
				var resp models.ShortenResponse
				err := json.NewDecoder(result.Body).Decode(&resp)
				require.NoError(t, err)
				assert.NotEmpty(t, resp.Result)
				assert.Contains(t, resp.Result, "http://localhost:8080/")
			}

			if tt.want.checkError {
				bodyBytes, err := io.ReadAll(result.Body)
				require.NoError(t, err)
				assert.NotEmpty(t, string(bodyBytes))
			}
		})
	}
}
