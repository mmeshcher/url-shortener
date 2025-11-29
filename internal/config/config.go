package config

import (
	"flag"
	"fmt"
)

type Config struct {
	ServerAddress string
	BaseURL       string
}

func ParseFlags() *Config {
	cfg := &Config{}
	flag.StringVar(&cfg.ServerAddress, "a", "localhost:8080", "Address of the server")
	flag.StringVar(&cfg.BaseURL, "b", "http://localhost:8080", "Base URL for short URLs")

	flag.Parse()

	return cfg
}

func (c *Config) Validate() error {
	if c.ServerAddress == "" {
		return fmt.Errorf("Server address cannot be empty")
	}
	if c.BaseURL == "" {
		return fmt.Errorf("Base URL cannot be empty")
	}
	return nil
}
