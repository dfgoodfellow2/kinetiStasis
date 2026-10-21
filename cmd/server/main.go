package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/api"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/config"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/db"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/middleware"
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

	// Shut down cleanly on SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	select {
	case err := <-serverErr:
		if err != nil {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		slog.Info("shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("server shutdown", "err", err)
		}
	}

	// Stop rate limiter cleanup goroutines now that the server is done.
	middleware.CloseRateLimiters()
	slog.Info("server stopped")
}
