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
}

func ParseFlags() (*Config, error) {
	cfg := &Config{}

	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse environment variables: %w", err)
	}

	envServerAddress := cfg.ServerAddress
	envBaseURL := cfg.BaseURL
	envFileStoragePath := cfg.FileStoragePath

	flag.StringVar(&cfg.ServerAddress, "a", "localhost:8080", "Address of the server")
	flag.StringVar(&cfg.BaseURL, "b", "http://localhost:8080", "Base URL for short URLs")
	flag.StringVar(&cfg.FileStoragePath, "file-storage-path", "url_storage.json", "Path to file storage")

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
