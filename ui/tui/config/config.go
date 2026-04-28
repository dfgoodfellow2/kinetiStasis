package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	BaseURL    string
	SessionDir string
}

func Load() *Config {
	baseURL := os.Getenv("DIET_API_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	home, _ := os.UserHomeDir()
	sessionDir := filepath.Join(home, ".config", "diet-tracker-v2")
	return &Config{BaseURL: baseURL, SessionDir: sessionDir}
}
