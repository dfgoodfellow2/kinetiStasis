package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/api"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/config"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/db"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/store"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/web"
)

func main() {
	// Structured logger — JSON in prod, human-readable text in dev
	var logHandler slog.Handler
	logHandler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(logHandler))

	// Load config from environment
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config error", "err", err)
		os.Exit(1)
	}

	if cfg.IsProd() {
		// Switch to JSON logging in production
		logHandler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
		slog.SetDefault(slog.New(logHandler))
	}

	// Open and migrate SQLite database
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		slog.Error("database error", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	// Create store wrapper around raw DB and build router with all middleware and routes.
	s := store.NewSQLiteStore(database)
	// Pass web.Handler() which will return nil in non-pwa builds (internal/web handles that).
	router := api.NewRouter(cfg, s, web.Handler())

	addr := ":" + cfg.Port
	slog.Info("server starting", "addr", addr, "env", cfg.Env, "db", cfg.DBPath)

	server := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
