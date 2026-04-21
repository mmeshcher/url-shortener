// Package models contains aliases to the domain models for backward compatibility.
package models

import "github.com/mmeshcher/url-shortener/internal/models/domain"

type ShortenRequest = domain.ShortenRequest
type ShortenResponse = domain.ShortenResponse
type URLRecord = domain.URLRecord
type BatchRequest = domain.BatchRequest
type BatchResponse = domain.BatchResponse
type UserURL = domain.UserURL
type Storage = domain.Storage
type UserURLsResponse = domain.UserURLsResponse
type DeleteRequest = domain.DeleteRequest
type BatchItem = domain.BatchItem
