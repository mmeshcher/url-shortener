package handler_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/mmeshcher/url-shortener/internal/audit"
	"github.com/mmeshcher/url-shortener/internal/handler"
	"github.com/mmeshcher/url-shortener/internal/middleware"
	"github.com/mmeshcher/url-shortener/internal/service"
	"go.uber.org/zap"
)

// ExampleHandler_ShortenHandler demonstrates how to use the ShortenHandler.
func ExampleHandler_ShortenHandler() {
	// Setup dependencies
	logger := zap.NewNop()
	auditor := audit.NewAuditor()
	authMiddleware := middleware.NewAuthMiddleware("secret-key", logger)
	shortenerService := service.NewShortenerService("http://localhost:8080", "", logger, "")
	h := handler.NewHandler(shortenerService, logger, authMiddleware, auditor)
	router := h.SetupRouter()

	// Create request
	originalURL := "https://yandex.ru"
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(originalURL))
	req.Header.Set("Content-Type", "text/plain")

	// Create a recorder to capture the response
	w := httptest.NewRecorder()

	// Perform request
	router.ServeHTTP(w, req)

	// Output result
	fmt.Printf("Status Code: %d\n", w.Code)
	fmt.Printf("Content-Type: %s\n", w.Header().Get("Content-Type"))
	// Note: We don't print the body because the short URL ID is random.

	// Output:
	// Status Code: 201
	// Content-Type: text/plain
}

// ExampleHandler_ShortenJSONHandler demonstrates how to use the ShortenJSONHandler.
func ExampleHandler_ShortenJSONHandler() {
	// Setup dependencies
	logger := zap.NewNop()
	auditor := audit.NewAuditor()
	authMiddleware := middleware.NewAuthMiddleware("secret-key", logger)
	shortenerService := service.NewShortenerService("http://localhost:8080", "", logger, "")
	h := handler.NewHandler(shortenerService, logger, authMiddleware, auditor)
	router := h.SetupRouter()

	// Create JSON request
	jsonBody := `{"url": "https://google.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewBufferString(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	// Create a recorder
	w := httptest.NewRecorder()

	// Perform request
	router.ServeHTTP(w, req)

	// Output result
	fmt.Printf("Status Code: %d\n", w.Code)
	fmt.Printf("Content-Type: %s\n", w.Header().Get("Content-Type"))

	// Output:
	// Status Code: 201
	// Content-Type: application/json
}

// ExampleHandler_PingHandler demonstrates how to use the PingHandler.
func ExampleHandler_PingHandler() {
	// Setup dependencies
	logger := zap.NewNop()
	auditor := audit.NewAuditor()
	authMiddleware := middleware.NewAuthMiddleware("secret-key", logger)
	shortenerService := service.NewShortenerService("http://localhost:8080", "", logger, "")
	h := handler.NewHandler(shortenerService, logger, authMiddleware, auditor)
	router := h.SetupRouter()

	// Create request
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)

	// Create a recorder
	w := httptest.NewRecorder()

	// Perform request
	router.ServeHTTP(w, req)

	// Output result
	fmt.Printf("Status Code: %d\n", w.Code)

	// Output:
	// Status Code: 200
}
