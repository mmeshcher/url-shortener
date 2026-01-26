package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mmeshcher/url-shortener/internal/models"
	"github.com/mmeshcher/url-shortener/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestShortenJSONHandler(t *testing.T) {
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
			want: want{
				statusCode: http.StatusMethodNotAllowed,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop()
			service := service.NewShortenerService("http://localhost:8080", "", logger, "")
			h := NewHandler(service, logger)
			router := h.SetupRouter()

			if tt.want.expectConflict {
				firstReq := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
				for key, value := range tt.headers {
					firstReq.Header.Set(key, value)
				}
				firstW := httptest.NewRecorder()
				router.ServeHTTP(firstW, firstReq)

				assert.Equal(t, http.StatusCreated, firstW.Code)
			}

			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))

			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

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
