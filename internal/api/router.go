package api

import (
	"database/sql"
	"net/http"
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
		// health needs raw db ping; try to extract from store
		var db *sql.DB
		if sdb, ok := s.(*store.SQLiteStore); ok {
			db = sdb.DB()
		}
		r.Get("/health", healthHandler(db))

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
			r.Get("/dashboard", calcH.Dashboard)

			// Check-in
			checkinH := handlers.NewCheckinHandler(s)
			r.Get("/checkin", checkinH.Preview)
			r.Post("/checkin", checkinH.Create)

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

func healthHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if err := db.PingContext(ctx); err != nil {
			respond.Error(w, http.StatusServiceUnavailable, "database unavailable")
			return
		}
		respond.JSON(w, http.StatusOK, map[string]any{
			"status":  "ok",
			"version": "2.0.0",
			"time":    time.Now().UTC().Format(time.RFC3339),
		})
	}
}
