// Package domain contains core data structures used throughout the application.
package domain

// ShortenRequest represents a request to shorten a single URL.
type ShortenRequest struct {
	URL string `json:"url"` // The original long URL to be shortened.
}

// ShortenResponse represents the result of shortening a single URL.
type ShortenResponse struct {
	Result string `json:"result"` // The resulting short URL.
}

// URLRecord represents a record of a single URL in storage.
type URLRecord struct {
	UUID        string `json:"uuid"`         // Unique identifier for the record.
	ShortURL    string `json:"short_url"`    // The short URL ID.
	OriginalURL string `json:"original_url"` // The original long URL.
}

// BatchRequest represents a single item in a batch shortening request.
type BatchRequest struct {
	CorrelationID string `json:"correlation_id"` // Client-provided ID to correlate request with response.
	OriginalURL   string `json:"original_url"`   // The original long URL to be shortened.
}

// BatchResponse represents a single item in a batch shortening response.
type BatchResponse struct {
	CorrelationID string `json:"correlation_id"` // Correlated client-provided ID.
	ShortURL      string `json:"short_url"`      // The resulting short URL.
}

// UserURL represents a pair of short and original URLs associated with a user.
type UserURL struct {
	ShortURL    string `json:"short_url"`    // The short URL.
	OriginalURL string `json:"original_url"` // The original long URL.
}

// Storage represents the database schema for URL storage.
type Storage struct {
	UUID        string `db:"uuid"`                 // Primary key.
	UserID      string `db:"user_id"`              // ID of the user who created the short URL.
	ShortURL    string `db:"short_url"`            // The short URL ID.
	OriginalURL string `db:"original_url"`         // The original long URL.
	IsDeleted   bool   `db:"is_deleted"`           // Flag indicating if the URL has been deleted.
	CreatedAt   string `db:"created_at,omitempty"` // Timestamp of creation.
}

// BatchItem represents a single item for batch repository operations.
type BatchItem struct {
	ShortID     string
	OriginalURL string
	UserID      string
}

// UserURLsResponse is a list of UserURL items.
type UserURLsResponse []UserURL

// DeleteRequest is a list of short URL IDs to be marked as deleted.
type DeleteRequest []string
