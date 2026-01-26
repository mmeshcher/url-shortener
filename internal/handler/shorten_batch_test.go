package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mmeshcher/url-shortener/internal/models"
	"github.com/mmeshcher/url-shortener/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestShortenBatchHandler(t *testing.T) {
	logger := zap.NewNop()
	service := service.NewShortenerService("http://localhost:8080", "", logger, "")
	h := NewHandler(service, logger)
	router := h.SetupRouter()

	tests := []struct {
		name       string
		method     string
		path       string
		body       []models.BatchRequest
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
			wantStatus: http.StatusCreated,
		},
		{
			name:       "negative: empty batch",
			method:     http.MethodPost,
			path:       "/api/shorten/batch",
			body:       []models.BatchRequest{},
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
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "negative: wrong method",
			method:     http.MethodGet,
			path:       "/api/shorten/batch",
			body:       []models.BatchRequest{},
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodyBytes, err := json.Marshal(tt.body)
			require.NoError(t, err)

			req := httptest.NewRequest(tt.method, tt.path, bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")

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
