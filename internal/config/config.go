package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/caarlos0/env/v6"
)

type Config struct {
	ServerAddress   string `env:"SERVER_ADDRESS" json:"server_address"`
	BaseURL         string `env:"BASE_URL" json:"base_url"`
	FileStoragePath string `env:"FILE_STORAGE_PATH" json:"file_storage_path"`
	DatabaseDSN     string `env:"DATABASE_DSN" json:"database_dsn"`
	SecretKey       string `env:"SECRET_KEY" json:"secret_key"`
	AuditFile       string `env:"AUDIT_FILE" json:"audit_file"`
	AuditURL        string `env:"AUDIT_URL" json:"audit_url"`
	EnableHTTPS     bool   `env:"ENABLE_HTTPS" json:"enable_https"`
	ConfigPath      string `env:"CONFIG"`
}

func ParseFlags() (*Config, error) {
	cfg := &Config{}

	var configPath string
	flag.StringVar(&configPath, "c", "", "Path to configuration file")
	flag.StringVar(&configPath, "config", "", "Path to configuration file")

	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse environment variables: %w", err)
	}
	if cfg.ConfigPath != "" && configPath == "" {
		configPath = cfg.ConfigPath
	}

	fServerAddress := flag.String("a", "", "Address of the server")
	fBaseURL := flag.String("b", "", "Base URL for short URLs")
	fFileStoragePath := flag.String("file-storage-path", "", "Path to file storage")
	fDatabaseDSN := flag.String("d", "", "Database connection string")
	fSecretKey := flag.String("secret-key", "", "Secret key for signing cookies")
	fAuditFile := flag.String("audit-file", "", "Path to audit file")
	fAuditURL := flag.String("audit-url", "", "URL for audit remote server")
	fEnableHTTPS := flag.Bool("s", false, "Enable HTTPS")

	flag.Parse()

	if configPath == "" {
	}

	if configPath != "" {
		jsonCfg, err := loadConfigFromJSON(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load config from JSON: %w", err)
		}
		cfg.mergeJSON(jsonCfg)
	}

	if *fServerAddress != "" {
		cfg.ServerAddress = *fServerAddress
	}
	if *fBaseURL != "" {
		cfg.BaseURL = *fBaseURL
	}
	if *fFileStoragePath != "" {
		cfg.FileStoragePath = *fFileStoragePath
	}
	if *fDatabaseDSN != "" {
		cfg.DatabaseDSN = *fDatabaseDSN
	}
	if *fSecretKey != "" {
		cfg.SecretKey = *fSecretKey
	}
	if *fAuditFile != "" {
		cfg.AuditFile = *fAuditFile
	}
	if *fAuditURL != "" {
		cfg.AuditURL = *fAuditURL
	}
	if *fEnableHTTPS {
		cfg.EnableHTTPS = *fEnableHTTPS
	}

	cfg.applyDefaultValues()

	return cfg, nil
}

func loadConfigFromJSON(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) mergeJSON(other *Config) {
	if c.ServerAddress == "" {
		c.ServerAddress = other.ServerAddress
	}
	if c.BaseURL == "" {
		c.BaseURL = other.BaseURL
	}
	if c.FileStoragePath == "" {
		c.FileStoragePath = other.FileStoragePath
	}
	if c.DatabaseDSN == "" {
		c.DatabaseDSN = other.DatabaseDSN
	}
	if c.SecretKey == "" {
		c.SecretKey = other.SecretKey
	}
	if c.AuditFile == "" {
		c.AuditFile = other.AuditFile
	}
	if c.AuditURL == "" {
		c.AuditURL = other.AuditURL
	}
	if !c.EnableHTTPS {
		c.EnableHTTPS = other.EnableHTTPS
	}
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
