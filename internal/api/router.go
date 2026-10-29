package api

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/auth"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/config"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/handlers"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/middleware"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/respond"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/store"

	geminiSvc "github.com/dfgoodfellow2/diet-tracker/v2/internal/services/gemini"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"golang.org/x/time/rate"
)

// NewRouter builds and returns the complete Chi router with all middleware
// and routes wired up.
func NewRouter(cfg *config.Config, s store.Store, webHandler http.Handler) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.Recover)
	r.Use(middleware.Logger)
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)

	// CORS — in prod use the configured domain; in dev allow localhost ports
	var origins []string
	if cfg.IsProd() {
		origins = []string{cfg.AppDomain}
	} else {
		origins = []string{"http://localhost:5173", "http://localhost:8080"}
	}
	r.Use(middleware.CORS(origins))
	r.Use(middleware.SecureHeaders)

	// Rate limiters
	stdLimiter := middleware.RateLimit(rate.Every(time.Minute/100), 20) // 100 req/min, burst 20
	parseLimiter := middleware.RateLimit(rate.Every(time.Minute/10), 3) // 10 req/min, burst 3

	// Handler constructors
	authH := handlers.NewAuthHandler(s, cfg)
	adminH := handlers.NewAdminHandler(s)
	profileH := handlers.NewProfileHandler(s)
	nutritionH := handlers.NewNutritionHandler(s)
	biometricH := handlers.NewBiometricHandler(s)
	workoutsH := handlers.NewWorkoutHandler(s)
	targetsH := handlers.NewTargetsHandler(s)
	mealsH := handlers.NewMealsHandler(s)
	groupsH := handlers.NewGroupsHandler(s)
	measurementsH := handlers.NewMeasurementsHandler(s)
	calcH := handlers.NewCalcHandler(s)
	exportH := handlers.NewExportHandler(s)

	var geminiClient *geminiSvc.Client
	if cfg.GeminiKey != "" {
		geminiClient = geminiSvc.NewClient(cfg.GeminiKey)
	}
	// Parse handler uses store interface now
	parseH := handlers.NewParseHandler(s, geminiClient)

	r.Route("/v1", func(r chi.Router) {
		// Apply standard rate limiter to all /v1 routes
		r.Use(stdLimiter)

		// Public
		// health uses store.PingContext now
		r.Get("/health", healthHandlerFunc(s, cfg))

		// Debug endpoints removed — kept health endpoint for checks

		// Auth routes (public)
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", authH.Register)
			r.Post("/login", authH.Login)
			r.Post("/refresh", authH.Refresh)
			r.Post("/logout", authH.Logout)
		})

		// Protected routes — require valid access token cookie
		r.Group(func(r chi.Router) {
			r.Use(auth.RequireAuth(cfg.JWTSecret))

			// Current user info
			r.Get("/auth/me", authH.Me)

			// Admin routes — also require is_admin=true
			r.Route("/admin", func(r chi.Router) {
				r.Use(auth.RequireAdmin)
				r.Get("/users", adminH.ListUsers)
				r.Delete("/users/{userID}", adminH.DeleteUser)
				r.Post("/users/{userID}/promote", adminH.PromoteUser)
				r.Post("/users/{userID}/demote", adminH.DemoteUser)
			})

			r.Get("/profile", profileH.Get)
			r.Put("/profile", profileH.Update)

			// Nutrition logs
			r.Get("/nutrition/logs", nutritionH.List)
			r.Get("/nutrition/logs/{date}", nutritionH.Get)
			r.Post("/nutrition/logs", nutritionH.Create)
			r.Put("/nutrition/logs/{date}", nutritionH.Update)
			r.Delete("/nutrition/logs/{date}", nutritionH.Delete)

			// Biometrics
			r.Get("/biometrics", biometricH.List)
			r.Get("/biometrics/{date}", biometricH.Get)
			r.Post("/biometrics", biometricH.Create)
			r.Put("/biometrics/{date}", biometricH.Update)
			r.Delete("/biometrics/{date}", biometricH.Delete)

			// Workouts
			r.Get("/workouts", workoutsH.List)
			r.Get("/workouts/{date}/{slot}", workoutsH.Get)
			r.Post("/workouts", workoutsH.Create)
			r.Put("/workouts/{date}/{slot}", workoutsH.Update)
			r.Delete("/workouts/{date}/{slot}", workoutsH.Delete)

			// Targets
			r.Get("/targets", targetsH.Get)
			r.Put("/targets", targetsH.Update)
			r.Get("/targets/history", targetsH.History)

			// Meals & templates (saved meals)
			r.Get("/meals/saved", mealsH.ListSaved)
			r.Post("/meals/saved", mealsH.CreateSaved)
			r.Delete("/meals/saved/{id}", mealsH.DeleteSaved)
			r.Get("/meals/templates", mealsH.ListTemplates)
			r.Post("/meals/templates", mealsH.CreateTemplate)
			r.Delete("/meals/templates/{id}", mealsH.DeleteTemplate)
			r.Post("/meals/templates/{id}/share", groupsH.ShareTemplate)
			r.Delete("/meals/templates/{id}/share", groupsH.UnshareTemplate)

			// Groups
			r.Post("/groups", groupsH.Create)
			r.Get("/groups", groupsH.List)
			r.Post("/groups/invites", groupsH.CreateInvite)
			// Joining by invite is a guessable-code surface, so it shares the
			// stricter limiter used by AI parsing.
			r.Group(func(r chi.Router) {
				r.Use(parseLimiter)
				r.Post("/groups/join", groupsH.Join)
			})
			r.Get("/groups/{id}/summary", groupsH.Summary)
			r.Patch("/groups/{id}/my-sharing", groupsH.UpdateMySharing)
			r.Delete("/groups/{id}/members/{userId}", groupsH.RemoveMember)
			r.Post("/groups/{id}/leave", groupsH.Leave)
			r.Delete("/groups/{id}", groupsH.Delete)

			// Measurements
			r.Get("/measurements", measurementsH.List)
			r.Post("/measurements", measurementsH.Create)
			r.Put("/measurements/{date}", measurementsH.Update)
			r.Delete("/measurements/{date}", measurementsH.Delete)

			// Calculations
			r.Get("/calc/tdee", calcH.TDEE)
			r.Get("/calc/macros", calcH.Macros)
			r.Get("/calc/readiness", calcH.Readiness)
			r.Get("/calc/bodyfat", calcH.BodyFat)
			r.Post("/calc/macros/distribute", calcH.DistributeMacros)
			r.Get("/dashboard", calcH.Dashboard)

			// Check-in
			checkinH := handlers.NewCheckinHandler(s)
			r.Get("/checkin", checkinH.Preview)
			r.Post("/checkin", checkinH.Create)
			r.Get("/checkin/history", checkinH.History)

			// AI parsing — strict rate limit
			r.Route("/parse", func(r chi.Router) {
				r.Use(parseLimiter)
				r.Post("/meal", parseH.Meal)
				r.Post("/workout", parseH.Workout)
			})

			// Export
			r.Get("/export/nutrition", exportH.Nutrition)
			r.Get("/export/workouts", exportH.Workouts)
			r.Get("/export/combined", exportH.Combined)
		})
	})

	// SPA — serve PWA for all non-API routes as a catch-all
	if webHandler != nil {
		// Catch all GET requests not handled by /v1 and serve the web app
		r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
			webHandler.ServeHTTP(w, r)
		})
		r.Head("/*", func(w http.ResponseWriter, r *http.Request) {
			webHandler.ServeHTTP(w, r)
		})
	}

	return r
}

// replicaMetricsTimeout caps the best-effort Litestream metrics lookup. Health
// must stay fast even when Litestream is not running or the metrics listener is
// wedged, so this is deliberately the only network call in the handler.
const replicaMetricsTimeout = 500 * time.Millisecond

// healthHandlerFunc reports liveness plus two best-effort operational signals:
//
//	dbModifiedAgoSec — age of the SQLite file's mtime (-1 when it cannot be read)
//	replicaAgeSec    — age of the most recent replica write, derived from a
//	                   Prometheus timestamp metric (-1 when unavailable)
//
// Neither probe can fail the health check; they are observability only.
func healthHandlerFunc(s store.Store, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if err := s.PingContext(ctx); err != nil {
			respond.Error(w, http.StatusServiceUnavailable, "database unavailable")
			return
		}

		dbPath, metricsURL := "", ""
		if cfg != nil {
			dbPath = cfg.DBPath
			metricsURL = cfg.LitestreamMetricsAddr
		}

		respond.JSON(w, http.StatusOK, map[string]any{
			"status":           "ok",
			"version":          "2.0.0",
			"time":             time.Now().UTC().Format(time.RFC3339),
			"dbModifiedAgoSec": dbModifiedAgoSec(dbPath),
			"replicaAgeSec":    replicaAgeSec(ctx, metricsURL),
		})
	}
}

// dbModifiedAgoSec returns how many seconds ago path was last modified, or -1
// if the file cannot be stat'd (for example before the first write or when
// DB_PATH points somewhere that does not exist in this environment).
func dbModifiedAgoSec(path string) int {
	if path == "" {
		return -1
	}
	info, err := os.Stat(path)
	if err != nil {
		return -1
	}
	ago := int(time.Since(info.ModTime()).Seconds())
	if ago < 0 {
		ago = 0
	}
	return ago
}

// replicaAgeSec fetches Litestream's Prometheus endpoint and, if it exposes a
// timestamp metric for the last replica write/sync, returns the number of
// seconds since that write. It returns -1 when the endpoint is unset,
// unreachable, slow (hard 500 ms cap), or when no such metric is present.
//
// Investigation of the vendored/upstream litestream v0.3.13 source
// (cmd/litestream/replicate.go, db.go, replica.go): the metrics listener is
// promhttp.Handler() over the default registry, but the daemon only publishes
// counters/gauges — litestream_db_size, litestream_wal_size,
// litestream_total_wal_bytes, litestream_shadow_wal_index,
// litestream_shadow_wal_size, litestream_sync_count, litestream_sync_error_count,
// litestream_sync_seconds, litestream_checkpoint_*, litestream_replica_wal_bytes,
// litestream_replica_wal_index, litestream_replica_wal_offset and
// litestream_replica_validation_total. None carries a wall-clock timestamp, and
// (contrary to the task note) v0.3.13 does not bind :9090 unless `addr` is set
// in litestream.yml, so replicaAgeSec is normally -1 today. The parser below is
// future-proofed for a timestamp gauge if one is added.
func replicaAgeSec(parent context.Context, metricsURL string) int {
	if metricsURL == "" {
		return -1
	}

	ctx, cancel := context.WithTimeout(parent, replicaMetricsTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, metricsURL, nil)
	if err != nil {
		return -1
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return -1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return -1
	}

	scanner := bufio.NewScanner(io.LimitReader(resp.Body, 1<<20))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := fields[0]
		if i := strings.IndexByte(name, '{'); i >= 0 {
			name = name[:i]
		}
		if !isReplicaTimestampMetric(name) {
			continue
		}

		ts, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			continue
		}
		// Prometheus timestamps are seconds; accept ms/µs/ns defensively for
		// exporters that emit a different unit.
		switch {
		case ts > 1e17:
			ts /= 1e9
		case ts > 1e14:
			ts /= 1e6
		case ts > 1e12:
			ts /= 1e3
		}
		age := int(time.Since(time.Unix(int64(ts), 0)).Seconds())
		if age < 0 {
			age = 0
		}
		return age
	}

	return -1
}

// isReplicaTimestampMetric matches Prometheus names that plausibly represent
// the moment of the most recent replica write/sync.
func isReplicaTimestampMetric(name string) bool {
	if !strings.HasPrefix(name, "litestream") || !strings.Contains(name, "timestamp") {
		return false
	}
	return strings.Contains(name, "replica") ||
		strings.Contains(name, "sync") ||
		strings.Contains(name, "write")
}
