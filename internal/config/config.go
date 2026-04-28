package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	Port      string
	Env       string
	DBPath    string
	JWTSecret []byte
	GeminiKey string
	AppDomain string // e.g. "https://diet.example.com" — used for CORS in prod
}

// Load reads config from environment variables.
// Returns an error if any required variable is missing.
func Load() (*Config, error) {
	godotenv.Load()
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET environment variable is required")
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/diet.db"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	env := os.Getenv("ENV")
	if env == "" {
		env = "development"
	}

	appDomain := os.Getenv("APP_DOMAIN")
	if appDomain == "" && env == "production" {
		appDomain = "https://diet-tracker-v2.fly.dev"
	}

	return &Config{
		Port:      port,
		Env:       env,
		DBPath:    dbPath,
		JWTSecret: []byte(jwtSecret),
		GeminiKey: os.Getenv("GEMINI_API_KEY"),
		AppDomain: appDomain,
	}, nil
}

// IsProd returns true when running in production mode.
func (c *Config) IsProd() bool {
	return c.Env == "production"
}
