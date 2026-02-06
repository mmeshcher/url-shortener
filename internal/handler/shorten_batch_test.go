package handler

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mmeshcher/url-shortener/internal/middleware"
	"github.com/mmeshcher/url-shortener/internal/models"
	"github.com/mmeshcher/url-shortener/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestShortenBatchHandler(t *testing.T) {
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

	tests := []struct {
		name       string
		method     string
		path       string
		body       []models.BatchRequest
		userID     string
		wantStatus int
		checkCount bool
	}{
		{
			name:   "positive test with multiple URLs",
			method: http.MethodPost,
			path:   "/api/shorten/batch",
			body: []models.BatchRequest{
				{CorrelationID: "1", OriginalURL: "https://yandex.ru"},
				{CorrelationID: "2", OriginalURL: "https://google.com"},
				{CorrelationID: "3", OriginalURL: "https://github.com"},
			},
			userID:     "test-user-1",
			wantStatus: http.StatusCreated,
			checkCount: true,
		},
		{
			name:   "negative: duplicate URL conflict",
			method: http.MethodPost,
			path:   "/api/shorten/batch",
			body: []models.BatchRequest{
				{CorrelationID: "1", OriginalURL: "https://duplicate.yandex.ru"},
				{CorrelationID: "2", OriginalURL: "https://duplicate.yandex.ru"},
				{CorrelationID: "3", OriginalURL: "https://unique.com"},
			},
			userID:     "test-user-2",
			wantStatus: http.StatusCreated,
			checkCount: true,
		},
		{
			name:   "positive: 1 invalid URL in batch, another valid",
			method: http.MethodPost,
			path:   "/api/shorten/batch",
			body: []models.BatchRequest{
				{CorrelationID: "1", OriginalURL: "invalid-url"},
				{CorrelationID: "2", OriginalURL: "https://yandex.ru"},
			},
			userID:     "test-user-3",
			wantStatus: http.StatusCreated,
		},
		{
			name:       "negative: empty batch",
			method:     http.MethodPost,
			path:       "/api/shorten/batch",
			body:       []models.BatchRequest{},
			userID:     "test-user-4",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:   "negative: invalid URL in batch",
			method: http.MethodPost,
			path:   "/api/shorten/batch",
			body: []models.BatchRequest{
				{CorrelationID: "1", OriginalURL: "invalid-url"},
				{CorrelationID: "2", OriginalURL: ""},
			},
			userID:     "test-user-5",
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "negative: wrong method",
			method:     http.MethodGet,
			path:       "/api/shorten/batch",
			body:       []models.BatchRequest{},
			userID:     "test-user-6",
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := service.NewShortenerService("http://localhost:8080", "", logger, "")
			h := NewHandler(service, logger, authMiddleware)
			router := h.SetupRouter()

			testCookie := createTestCookie(tt.userID)

			bodyBytes, err := json.Marshal(tt.body)
			require.NoError(t, err)

			req := httptest.NewRequest(tt.method, tt.path, bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			req.AddCookie(testCookie)

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			result := w.Result()
			defer result.Body.Close()

			assert.Equal(t, tt.wantStatus, result.StatusCode)

			if tt.checkCount && tt.wantStatus == http.StatusCreated {
				var response []models.BatchResponse
				err := json.NewDecoder(result.Body).Decode(&response)
				require.NoError(t, err)
				assert.Len(t, response, len(tt.body))

				for i, respItem := range response {
					assert.Equal(t, tt.body[i].CorrelationID, respItem.CorrelationID)
					assert.Contains(t, respItem.ShortURL, "http://localhost:8080/")
				}
			}
		})
	}
}
