package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"io"

	"github.com/mmeshcher/url-shortener/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedirectHandler(t *testing.T) {
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
		setup  func() (*service.ShortenerService, string)
		want   want
	}{
		{
			name:   "positive test",
			method: http.MethodGet,
			setup: func() (*service.ShortenerService, string) {
				service := service.NewShortenerService("http://localhost:8080")
				shortURL := service.CreateShortURL("https://practicum.yandex.ru/")
				shortID := shortURL[len("http://localhost:8080/"):]
				return service, shortID
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
			setup: func() (*service.ShortenerService, string) {
				return service.NewShortenerService("http://localhost:8080"), "nonexistent123"
			},
			want: want{
				statusCode:  400,
				contentType: "text/plain; charset=utf-8",
				location:    "",
				body:        "Original URL not exists for this short URL\n",
				checkBody:   true,
			},
		},
		{
			name:   "negative: empty short URL",
			method: http.MethodGet,
			setup: func() (*service.ShortenerService, string) {
				return service.NewShortenerService("http://localhost:8080"), ""
			},
			want: want{
				statusCode:  400,
				contentType: "text/plain; charset=utf-8",
				location:    "",
				body:        "Bad Request\n",
				checkBody:   true,
			},
		},
		{
			name:   "negative: wrong method POST",
			method: http.MethodPost,
			setup: func() (*service.ShortenerService, string) {
				service := service.NewShortenerService("http://localhost:8080")
				shortURL := service.CreateShortURL("https://practicum.yandex.ru/")
				shortID := shortURL[len("http://localhost:8080/"):]
				return service, shortID
			},
			want: want{
				statusCode:  400,
				contentType: "text/plain; charset=utf-8",
				location:    "",
				body:        "Bad Request\n",
				checkBody:   true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, shortID := tt.setup()

			request := httptest.NewRequest(tt.method, "/"+shortID, nil)
			w := httptest.NewRecorder()

			h := NewHandler(service)
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
