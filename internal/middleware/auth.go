package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type contextKey string

const userIDKey contextKey = "userID"

const (
	cookieName    = "user_id"
	cookieExpires = 365 * 24 * time.Hour
)

type AuthMiddleware struct {
	secretKey []byte
	logger    *zap.Logger
}

func NewAuthMiddleware(secret string, logger *zap.Logger) *AuthMiddleware {
	return &AuthMiddleware{
		secretKey: []byte(secret),
		logger:    logger,
	}
}

func (a *AuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.secretKey == nil || a.logger == nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		userID, err := a.getOrCreateUserID(r)
		if err != nil {
			a.logger.Error("Failed to get or create user ID", zap.Error(err))
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		cookie, err := r.Cookie(cookieName)
		if err != nil || !a.validateCookie(cookie.Value, userID) {
			a.setUserCookie(w, userID)
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, userIDKey, userID)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

func (a *AuthMiddleware) getOrCreateUserID(r *http.Request) (string, error) {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		newUserID := uuid.New().String()
		return newUserID, nil
	}

	userID, valid := a.parseCookie(cookie.Value)
	if !valid {
		return "", errors.New("invalid cookie signature")
	}

	return userID, nil
}

func (a *AuthMiddleware) setUserCookie(w http.ResponseWriter, userID string) {
	signedValue := a.signUserID(userID)

	cookie := &http.Cookie{
		Name:     cookieName,
		Value:    signedValue,
		Path:     "/",
		Expires:  time.Now().Add(cookieExpires),
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	}

	http.SetCookie(w, cookie)
}

func (a *AuthMiddleware) signUserID(userID string) string {
	mac := hmac.New(sha256.New, a.secretKey)
	mac.Write([]byte(userID))
	signature := mac.Sum(nil)
	return userID + "." + hex.EncodeToString(signature)
}

func (a *AuthMiddleware) parseCookie(cookieValue string) (string, bool) {
	parts := strings.Split(cookieValue, ".")
	if len(parts) != 2 {
		return "", false
	}

	userID := parts[0]
	signature := parts[1]

	expectedSignature := a.signUserID(userID)
	expectedParts := strings.Split(expectedSignature, ".")
	if len(expectedParts) != 2 {
		return "", false
	}

	if !hmac.Equal([]byte(signature), []byte(expectedParts[1])) {
		return "", false
	}

	return userID, true
}

func (a *AuthMiddleware) validateCookie(cookieValue, expectedUserID string) bool {
	userID, valid := a.parseCookie(cookieValue)
	if !valid {
		return false
	}
	return userID == expectedUserID
}

func GetUserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(userIDKey).(string)
	return userID, ok
}
