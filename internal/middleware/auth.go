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

var (
	secretKey []byte
	logger    *zap.Logger
)

func InitAuthMiddleware(secret string, log *zap.Logger) {
	secretKey = []byte(secret)
	logger = log
}

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if secretKey == nil || logger == nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		userID, err := getOrCreateUserID(r)
		if err != nil {
			logger.Error("Failed to get or create user ID", zap.Error(err))
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		cookie, err := r.Cookie(cookieName)
		if err != nil || !validateCookie(cookie.Value, userID) {
			setUserCookie(w, userID)
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, userIDKey, userID)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

func getOrCreateUserID(r *http.Request) (string, error) {
	cookie, err := r.Cookie(cookieName)
	if err != nil {
		newUserID := uuid.New().String()
		return newUserID, nil
	}

	userID, valid := parseCookie(cookie.Value)
	if !valid {
		return "", errors.New("invalid cookie signature")
	}

	return userID, nil
}

func setUserCookie(w http.ResponseWriter, userID string) {
	signedValue := signUserID(userID)

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

func signUserID(userID string) string {
	mac := hmac.New(sha256.New, secretKey)
	mac.Write([]byte(userID))
	signature := mac.Sum(nil)
	return userID + "." + hex.EncodeToString(signature)
}

func parseCookie(cookieValue string) (string, bool) {
	parts := strings.Split(cookieValue, ".")
	if len(parts) != 2 {
		return "", false
	}

	userID := parts[0]
	signature := parts[1]

	expectedSignature := signUserID(userID)
	expectedParts := strings.Split(expectedSignature, ".")
	if len(expectedParts) != 2 {
		return "", false
	}

	if !hmac.Equal([]byte(signature), []byte(expectedParts[1])) {
		return "", false
	}

	return userID, true
}

func validateCookie(cookieValue, expectedUserID string) bool {
	userID, valid := parseCookie(cookieValue)
	if !valid {
		return false
	}
	return userID == expectedUserID
}

func GetUserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(userIDKey).(string)
	return userID, ok
}
