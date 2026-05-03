package api

import (
	"context"
	"net/http"

	"log/slog"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/respond"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/store"
)

// dbCheckAllHandler runs a set of representative queries and store method calls
// to help debug failing handlers. This is unauthenticated and intended for
// temporary local debugging only.
func dbCheckAllHandler(s store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// 1) List tables — if the store implements RawQueryTables (SQLite concrete
		// implementation provides this helper) use it.
		tables := []string{}
		if q, ok := s.(interface {
			RawQueryTables(context.Context) ([]string, error)
		}); ok {
			if t, err := q.RawQueryTables(ctx); err != nil {
				slog.Error("list tables failed", "err", err)
			} else {
				tables = t
			}
		} else {
			// Fallback: exercise a simple query via CountCheckInLogs to ensure DB is reachable.
			if c, ok := s.(interface {
				CountCheckInLogs(context.Context) (int, error)
			}); ok {
				if _, err := c.CountCheckInLogs(ctx); err != nil {
					slog.Error("CountCheckInLogs failed", "err", err)
				}
			}
		}

		// Hardcoded test user
		testUser := "00000000-0000-0000-0000-000000000000"

		// Call the key store methods and capture errors
		var profileErr, biometricErr, nutritionErr, targetsErr, lastCheckinErr error

		profileErr = func() error {
			_, err := s.FetchProfile(ctx, testUser)
			if err != nil {
				slog.Error("FetchProfile failed", "err", err, "user_id", testUser)
			}
			return err
		}()

		biometricErr = func() error {
			// use range methods where available; use a simple from/to
			_, err := s.FetchBiometricLogsRange(ctx, testUser, "1970-01-01", "2099-12-31")
			if err == nil {
				return nil
			}
			// fallback to non-range
			_, err2 := s.FetchBiometricLogs(ctx, testUser, "1970-01-01")
			if err2 != nil {
				slog.Error("FetchBiometricLogs failed", "err", err2, "user_id", testUser)
			}
			return err2
		}()

		nutritionErr = func() error {
			_, err := s.FetchNutritionLogsRange(ctx, testUser, "1970-01-01", "2099-12-31")
			if err == nil {
				return nil
			}
			_, err2 := s.FetchNutritionLogs(ctx, testUser, "1970-01-01")
			if err2 != nil {
				slog.Error("FetchNutritionLogs failed", "err", err2, "user_id", testUser)
			}
			return err2
		}()

		targetsErr = func() error {
			_, err := s.FetchTargets(ctx, testUser)
			if err != nil {
				slog.Error("FetchTargets failed", "err", err, "user_id", testUser)
			}
			return err
		}()

		lastCheckinErr = func() error {
			_, err := s.FetchLastCheckin(ctx, testUser)
			if err != nil {
				slog.Error("FetchLastCheckin failed", "err", err, "user_id", testUser)
			}
			return err
		}()

		resp := map[string]interface{}{
			"tables":             tables,
			"profile_error":      nil,
			"biometric_error":    nil,
			"nutrition_error":    nil,
			"targets_error":      nil,
			"last_checkin_error": nil,
		}
		if profileErr != nil {
			resp["profile_error"] = profileErr.Error()
		}
		if biometricErr != nil {
			resp["biometric_error"] = biometricErr.Error()
		}
		if nutritionErr != nil {
			resp["nutrition_error"] = nutritionErr.Error()
		}
		if targetsErr != nil {
			resp["targets_error"] = targetsErr.Error()
		}
		if lastCheckinErr != nil {
			resp["last_checkin_error"] = lastCheckinErr.Error()
		}

		respond.JSON(w, http.StatusOK, resp)
	}
}
