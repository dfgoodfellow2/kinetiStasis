package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

const (
	// DefaultGeminiModel is the text-only meal/workout parsing model.
	DefaultGeminiModel = "gemini-2.5-flash-lite"
	// DefaultGeminiVisionModel is the multimodal model used when a meal photo
	// is supplied to the parser.
	DefaultGeminiVisionModel = "gemini-2.5-flash"
	// DefaultLitestreamMetricsAddr is the local Prometheus endpoint used by
	// /v1/health to report replica freshness. Litestream only binds it when
	// `addr` is configured in litestream.yml; when absent the probe reports -1.
	DefaultLitestreamMetricsAddr = "http://127.0.0.1:9090/metrics"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	Port              string
	Env               string
	DBPath            string
	JWTSecret         []byte
	GeminiKey         string
	GeminiModel       string
	GeminiVisionModel string
	AppDomain         string // e.g. "https://diet.example.com" — used for CORS in prod
	// LitestreamMetricsAddr is the best-effort Prometheus endpoint polled by
	// /v1/health for replica age. Override with LITESTREAM_METRICS_ADDR.
	LitestreamMetricsAddr string
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

	geminiModel, geminiVisionModel := GeminiModels()

	return &Config{
		Port:              port,
		Env:               env,
		DBPath:            dbPath,
		JWTSecret:         []byte(jwtSecret),
		GeminiKey:         os.Getenv("GEMINI_API_KEY"),
		GeminiModel:       geminiModel,
		GeminiVisionModel: geminiVisionModel,
		AppDomain:         appDomain,
		// Optional: an empty value disables the probe, otherwise this caps at
		// 500 ms inside the health handler.
		LitestreamMetricsAddr: envOr("LITESTREAM_METRICS_ADDR", DefaultLitestreamMetricsAddr),
	}, nil
}

// GeminiModels resolves the configured text and vision model names.
// It is shared with the Gemini service constructor so that environments which
// build the service directly (rather than through Config) still honour the
// same GEMINI_MODEL / GEMINI_VISION_MODEL environment variables.
func GeminiModels() (textModel, visionModel string) {
	return envOr("GEMINI_MODEL", DefaultGeminiModel), envOr("GEMINI_VISION_MODEL", DefaultGeminiVisionModel)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// IsProd returns true when running in production mode.
func (c *Config) IsProd() bool {
	return c.Env == "production"
}
