package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testHandler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	defer r.Body.Close()

	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "text/plain"
	}
	w.Header().Set("Content-Type", contentType)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("received: " + string(body)))
}

func TestGzipMiddleware(t *testing.T) {
	type want struct {
		statusCode      int
		contentEncoding string
		contentType     string
		bodyContains    string
	}

	tests := []struct {
		name        string
		requestBody string
		headers     map[string]string
		want        want
	}{
		{
			name:        "positive: client accepts gzip, text/html",
			requestBody: "test request",
			headers: map[string]string{
				"Accept-Encoding": "gzip",
				"Content-Type":    "text/html",
			},
			want: want{
				statusCode:      http.StatusOK,
				contentEncoding: "gzip",
				contentType:     "text/html",
				bodyContains:    "received: test request",
			},
		},
		{
			name:        "positive: client accepts gzip, application/json",
			requestBody: `{"test":"data"}`,
			headers: map[string]string{
				"Accept-Encoding": "gzip",
				"Content-Type":    "application/json",
			},
			want: want{
				statusCode:      http.StatusOK,
				contentEncoding: "gzip",
				contentType:     "application/json",
				bodyContains:    `received: {"test":"data"}`,
			},
		},
		{
			name:        "negative: client doesn't accept gzip",
			requestBody: "test request",
			headers: map[string]string{
				"Accept-Encoding": "",
				"Content-Type":    "text/html",
			},
			want: want{
				statusCode:      http.StatusOK,
				contentEncoding: "",
				contentType:     "text/html",
				bodyContains:    "received: test request",
			},
		},
		{
			name:        "positive: compressed request body",
			requestBody: "compressed request",
			headers: map[string]string{
				"Content-Encoding": "gzip",
				"Accept-Encoding":  "gzip",
				"Content-Type":     "text/html",
			},
			want: want{
				statusCode:      http.StatusOK,
				contentEncoding: "gzip",
				contentType:     "text/html",
				bodyContains:    "received: compressed request",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requestBody io.Reader
			if strings.Contains(tt.headers["Content-Encoding"], "gzip") {
				var buf bytes.Buffer
				gz := gzip.NewWriter(&buf)
				_, err := gz.Write([]byte(tt.requestBody))
				require.NoError(t, err)
				err = gz.Close()
				require.NoError(t, err)
				requestBody = &buf
			} else {
				requestBody = strings.NewReader(tt.requestBody)
			}

			req := httptest.NewRequest("POST", "/test", requestBody)
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			w := httptest.NewRecorder()

			handler := GzipMiddleware(http.HandlerFunc(testHandler))
			handler.ServeHTTP(w, req)

			result := w.Result()
			defer result.Body.Close()

			assert.Equal(t, tt.want.statusCode, result.StatusCode)
			assert.Equal(t, tt.want.contentEncoding, result.Header.Get("Content-Encoding"))
			assert.Equal(t, tt.want.contentType, result.Header.Get("Content-Type"))

			var bodyBytes []byte
			var err error

			if result.Header.Get("Content-Encoding") == "gzip" {
				gzReader, gzErr := gzip.NewReader(result.Body)
				require.NoError(t, gzErr)
				defer gzReader.Close()
				bodyBytes, err = io.ReadAll(gzReader)
				require.NoError(t, err)
			} else {
				bodyBytes, err = io.ReadAll(result.Body)
				require.NoError(t, err)
			}

			assert.Contains(t, string(bodyBytes), tt.want.bodyContains)
		})
	}
}
