package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"io"

	"github.com/mmeshcher/url-shortener/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestShortenHandler(t *testing.T) {
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
		want    want
	}{
		{
			name:    "positive test",
			request: "/",
			body:    "https://practicum.yandex.ru/",
			method:  http.MethodPost,
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
			request := httptest.NewRequest(tt.method, tt.request, strings.NewReader(tt.body))
			request.Header.Set("Content-Type", "text/plain")
			w := httptest.NewRecorder()
			logger := zap.NewNop()
			service := service.NewShortenerService("http://localhost:8080", "", logger, "")
			h := NewHandler(service, logger)
			r := h.SetupRouter()

			if tt.name == "negative: duplicate URL conflict" {
				firstReq := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body))
				firstReq.Header.Set("Content-Type", "text/plain")
				firstW := httptest.NewRecorder()
				r.ServeHTTP(firstW, firstReq)

				r.ServeHTTP(w, request)
			} else {
				r.ServeHTTP(w, request)
			}

			result := w.Result()
			defer result.Body.Close()

			assert.Equal(t, tt.want.statusCode, result.StatusCode)
			assert.Equal(t, tt.want.contentType, result.Header.Get("Content-Type"))

			bodyResult, err := io.ReadAll(result.Body)
			require.NoError(t, err)
			bodyStr := string(bodyResult)

			if tt.want.checkBody {
				assert.Equal(t, tt.want.body, bodyStr)
			} else {
				assert.Contains(t, bodyStr, tt.want.body)
				assert.Greater(t, len(bodyStr), len(tt.want.body))
			}
		})
	}
}
