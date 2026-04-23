package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mmeshcher/url-shortener/internal/audit"
	"github.com/mmeshcher/url-shortener/internal/middleware"
	"github.com/mmeshcher/url-shortener/internal/models/domain"
	"github.com/mmeshcher/url-shortener/internal/repository"
	"github.com/mmeshcher/url-shortener/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestStatsHandler(t *testing.T) {
	logger := zap.NewNop()
	auditor := audit.NewAuditor()
	authMiddleware := middleware.NewAuthMiddleware("secret-key", logger)

	tests := []struct {
		name          string
		trustedSubnet string
		xRealIP       string
		wantStatus    int
		setupRepo     func(repo repository.URLRepository)
	}{
		{
			name:          "positive: access allowed",
			trustedSubnet: "192.168.1.0/24",
			xRealIP:       "192.168.1.10",
			wantStatus:    http.StatusOK,
			setupRepo: func(repo repository.URLRepository) {
				repo.SaveURL(context.Background(), "short1", "http://example.com", "user1")
				repo.SaveURL(context.Background(), "short2", "http://example.org", "user2")
			},
		},
		{
			name:          "negative: IP not in subnet",
			trustedSubnet: "192.168.1.0/24",
			xRealIP:       "10.0.0.1",
			wantStatus:    http.StatusForbidden,
		},
		{
			name:          "negative: missing X-Real-IP",
			trustedSubnet: "192.168.1.0/24",
			xRealIP:       "",
			wantStatus:    http.StatusForbidden,
		},
		{
			name:          "negative: empty trusted subnet",
			trustedSubnet: "",
			xRealIP:       "192.168.1.10",
			wantStatus:    http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := repository.NewMemoryRepository("", logger)
			if tt.setupRepo != nil {
				tt.setupRepo(repo)
			}

			s := service.NewShortenerService("http://localhost:8080", repo, logger)
			ipMiddleware := middleware.NewIPControlMiddleware(tt.trustedSubnet, logger)
			h := NewHandler(s, logger, authMiddleware, auditor, ipMiddleware)
			router := h.SetupRouter()

			req := httptest.NewRequest(http.MethodGet, "/api/internal/stats", nil)
			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			if tt.wantStatus == http.StatusOK {
				var resp domain.StatsResponse
				err := json.NewDecoder(w.Body).Decode(&resp)
				require.NoError(t, err)
				assert.Equal(t, 2, resp.URLs)
				assert.Equal(t, 2, resp.Users)
			}
		})
	}
}
