package config

import (
	"flag"
	"fmt"

	"github.com/caarlos0/env/v6"
)

type Config struct {
	ServerAddress   string `env:"SERVER_ADDRESS"`
	BaseURL         string `env:"BASE_URL"`
	FileStoragePath string `env:"FILE_STORAGE_PATH"`
	DatabaseDSN     string `env:"DATABASE_DSN"`
	SecretKey       string `env:"SECRET_KEY"`
	AuditFile       string `env:"AUDIT_FILE"`
	AuditURL        string `env:"AUDIT_URL"`
	EnableHTTPS     bool   `env:"ENABLE_HTTPS"`
}

func ParseFlags() (*Config, error) {
	cfg := &Config{}

	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse environment variables: %w", err)
	}

	envServerAddress := cfg.ServerAddress
	envBaseURL := cfg.BaseURL
	envFileStoragePath := cfg.FileStoragePath
	envDatabaseDSN := cfg.DatabaseDSN
	envSecretKey := cfg.SecretKey
	envAuditFile := cfg.AuditFile
	envAuditURL := cfg.AuditURL
	envEnableHTTPS := cfg.EnableHTTPS

	flag.StringVar(&cfg.ServerAddress, "a", "localhost:8080", "Address of the server")
	flag.StringVar(&cfg.BaseURL, "b", "http://localhost:8080", "Base URL for short URLs")
	flag.StringVar(&cfg.FileStoragePath, "file-storage-path", "url_storage.json", "Path to file storage")
	flag.StringVar(&cfg.DatabaseDSN, "d", "", "Database connection string")
	flag.StringVar(&cfg.SecretKey, "secret-key", "secret-key", "Secret key for signing cookies")
	flag.StringVar(&cfg.AuditFile, "audit-file", "", "Path to audit file")
	flag.StringVar(&cfg.AuditURL, "audit-url", "", "URL for audit remote server")
	flag.BoolVar(&cfg.EnableHTTPS, "s", false, "Enable HTTPS")

	flag.Parse()

	if envServerAddress != "" {
		cfg.ServerAddress = envServerAddress
	}
	if envBaseURL != "" {
		cfg.BaseURL = envBaseURL
	}
	if envFileStoragePath != "" {
		cfg.FileStoragePath = envFileStoragePath
	}
	if envDatabaseDSN != "" {
		cfg.DatabaseDSN = envDatabaseDSN
	}
	if envSecretKey != "" {
		cfg.SecretKey = envSecretKey
	}
	if envAuditFile != "" {
		cfg.AuditFile = envAuditFile
	}
	if envAuditURL != "" {
		cfg.AuditURL = envAuditURL
	}
	if envEnableHTTPS {
		cfg.EnableHTTPS = envEnableHTTPS
	}

	cfg.applyDefaultValues()

	return cfg, nil
}

func (c *Config) applyDefaultValues() {
	if c.ServerAddress == "" {
		c.ServerAddress = getDefaultServerAddress()
	}

	if c.BaseURL == "" {
		c.BaseURL = getDefaultBaseURL()
	}

	if c.FileStoragePath == "" {
		c.FileStoragePath = getDefaultFileStoragePath()
	}
}

func getDefaultServerAddress() string {
	return "localhost:8080"
}

func getDefaultBaseURL() string {
	return "http://localhost:8080"
}

func getDefaultFileStoragePath() string {
	return "url_storage.json"
}
