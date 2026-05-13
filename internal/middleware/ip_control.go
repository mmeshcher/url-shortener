package middleware

import (
	"net"
	"net/http"

	"go.uber.org/zap"
)

// IPControlMiddleware restricts access to handlers based on client IP and trusted subnet.
type IPControlMiddleware struct {
	trustedSubnet *net.IPNet
	logger        *zap.Logger
}

// NewIPControlMiddleware creates a new IPControlMiddleware instance.
func NewIPControlMiddleware(subnetStr string, logger *zap.Logger) *IPControlMiddleware {
	var subnet *net.IPNet
	if subnetStr != "" {
		_, parsedSubnet, err := net.ParseCIDR(subnetStr)
		if err != nil {
			logger.Error("Failed to parse trusted subnet CIDR", zap.String("subnet", subnetStr), zap.Error(err))
		} else {
			subnet = parsedSubnet
		}
	}

	return &IPControlMiddleware{
		trustedSubnet: subnet,
		logger:        logger,
	}
}

// Middleware checks if the X-Real-IP header is present and the IP is within the trusted subnet.
func (m *IPControlMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.trustedSubnet == nil {
			m.logger.Warn("Access denied: trusted_subnet is not configured or invalid")
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}

		ipStr := r.Header.Get("X-Real-IP")
		if ipStr == "" {
			m.logger.Warn("Access denied: X-Real-IP header is missing")
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}

		ip := net.ParseIP(ipStr)
		if ip == nil {
			m.logger.Warn("Access denied: invalid IP in X-Real-IP header", zap.String("ip", ipStr))
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}

		if !m.trustedSubnet.Contains(ip) {
			m.logger.Warn("Access denied: IP not in trusted subnet", zap.String("ip", ipStr), zap.String("subnet", m.trustedSubnet.String()))
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
