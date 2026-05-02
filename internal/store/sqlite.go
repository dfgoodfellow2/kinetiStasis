package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
)

// NewSQLiteStore wraps *sql.DB to implement Store interface.
type SQLiteStore struct{ db *sql.DB }

func NewSQLiteStore(db *sql.DB) *SQLiteStore { return &SQLiteStore{db: db} }

// Ensure interface
var _ Store = (*SQLiteStore)(nil)

// UserStore
func (s *SQLiteStore) FetchProfile(ctx context.Context, userID string) (models.Profile, error) {
	var p models.Profile
	err := s.db.QueryRowContext(ctx, `
        SELECT user_id, COALESCE(name,''), COALESCE(age,0), COALESCE(sex,''),
               COALESCE(height_cm,0), COALESCE(activity,'sedentary'), COALESCE(exercise_freq,0),
               COALESCE(running_km,0), COALESCE(is_lifter,0), COALESCE(goal,'maintenance'),
               COALESCE(prioritize_carbs,0), COALESCE(bf_pct,0), COALESCE(hr_rest,0),
               COALESCE(hr_max,0), COALESCE(grip_weight,0.5), COALESCE(tdee_lookback_days,90),
               COALESCE(sleep_quality_max,10.0), COALESCE(units,'imperial'), updated_at
        FROM profiles WHERE user_id = ?`, userID,
	).Scan(&p.UserID, &p.Name, &p.Age, &p.Sex, &p.HeightCm, &p.Activity, &p.ExerciseFreq,
		&p.RunningKm, &p.IsLifter, &p.Goal, &p.PrioritizeCarbs, &p.BfPct, &p.HRRest,
		&p.HRMax, &p.GripWeight, &p.TDEELookbackDays, &p.SleepQualityMax, &p.Units, &p.UpdatedAt)
	return p, err
}

// LogStore
func (s *SQLiteStore) FetchNutritionLogs(ctx context.Context, userID, since string) ([]models.NutritionLog, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, user_id, date, calories, protein_g, carbs_g, fat_g, fiber_g, water_ml, COALESCE(meal_notes,''), updated_at FROM nutrition_logs WHERE user_id=? AND date >= ? ORDER BY date ASC`, userID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.NutritionLog
	for rows.Next() {
		var n models.NutritionLog
		if err := rows.Scan(&n.ID, &n.UserID, &n.Date, &n.Calories, &n.ProteinG, &n.CarbsG, &n.FatG, &n.FiberG, &n.WaterMl, &n.MealNotes, &n.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}

func (s *SQLiteStore) FetchBiometricLogs(ctx context.Context, userID, since string) ([]models.BiometricLog, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, user_id, date, COALESCE(weight_kg,0), COALESCE(waist_cm,0), COALESCE(grip_kg,0), COALESCE(bolt_score,0), COALESCE(sleep_hours,0), COALESCE(sleep_quality,0), COALESCE(subjective_feel,0), COALESCE(body_fat_pct,0), COALESCE(notes,''), updated_at FROM biometric_logs WHERE user_id=? AND date >= ? ORDER BY date ASC`, userID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.BiometricLog
	for rows.Next() {
		var b models.BiometricLog
		if err := rows.Scan(&b.ID, &b.UserID, &b.Date, &b.WeightKg, &b.WaistCm, &b.GripKg, &b.BoltScore, &b.SleepHours, &b.SleepQuality, &b.SubjectiveFeel, &b.BodyFatPct, &b.Notes, &b.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, nil
}

func (s *SQLiteStore) FetchLatestWeight(ctx context.Context, userID string) (float64, error) {
	var w float64
	err := s.db.QueryRowContext(ctx, `SELECT weight_kg FROM biometric_logs WHERE user_id=? AND weight_kg > 0 ORDER BY date DESC LIMIT 1`, userID).Scan(&w)
	if err != nil {
		return 0, err
	}
	return w, nil
}

// FetchBodyMeasurements returns the most recent neck/waist/hips measurements for a user.
func (s *SQLiteStore) FetchBodyMeasurements(ctx context.Context, userID string) (neckCm, waistCm, hipsCm float64, err error) {
	row := s.db.QueryRowContext(ctx, `SELECT COALESCE(neck_cm,0), COALESCE(waist_cm,0), COALESCE(hips_cm,0) FROM body_measurements WHERE user_id=? ORDER BY date DESC LIMIT 1`, userID)
	if err := row.Scan(&neckCm, &waistCm, &hipsCm); err != nil {
		if err == sql.ErrNoRows {
			return 0, 0, 0, nil
		}
		return 0, 0, 0, err
	}
	return neckCm, waistCm, hipsCm, nil
}

// FetchLastCheckin returns the most recent check_in_date for a user (YYYY-MM-DD).
func (s *SQLiteStore) FetchLastCheckin(ctx context.Context, userID string) (lastCheckinDate string, err error) {
	row := s.db.QueryRowContext(ctx, `SELECT check_in_date FROM check_in_logs WHERE user_id = ? ORDER BY check_in_date DESC LIMIT 1`, userID)
	if err := row.Scan(&lastCheckinDate); err != nil {
		if err == sql.ErrNoRows {
			return "", nil // No check-in yet — not an error
		}
		return "", err
	}
	return lastCheckinDate, nil
}

// FetchGripPB returns the maximum grip_kg for the user in the 120 days before `today`.
func (s *SQLiteStore) FetchGripPB(ctx context.Context, userID, today string) (float64, error) {
	var gripPB float64
	row := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(grip_kg),0) FROM biometric_logs WHERE user_id = ? AND grip_kg > 0 AND date >= DATE(?, '-120 days')`, userID, today)
	if err := row.Scan(&gripPB); err != nil {
		return 0, err
	}
	return gripPB, nil
}

// WorkoutStore
func (s *SQLiteStore) FetchWorkouts(ctx context.Context, userID, since string) ([]models.WorkoutEntry, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, user_id, date, slot, COALESCE(title,''), COALESCE(raw_notes,''), COALESCE(duration_min,0), COALESCE(calories_burned,0), COALESCE(mwv,0), COALESCE(nds,0), COALESCE(session_density,0), COALESCE(exercises_json,'[]'), COALESCE(metadata_json,'{}'), updated_at FROM workouts WHERE user_id=? AND date >= ? ORDER BY date ASC`, userID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.WorkoutEntry
	for rows.Next() {
		var w models.WorkoutEntry
		var exercisesJSON, metadataJSON string
		if err := rows.Scan(&w.ID, &w.UserID, &w.Date, &w.Slot, &w.Title, &w.RawNotes, &w.DurationMin, &w.CaloriesBurned, &w.MWV, &w.NDS, &w.SessionDensity, &exercisesJSON, &metadataJSON, &w.UpdatedAt); err != nil {
			return nil, err
		}
		if exercisesJSON != "" && exercisesJSON != "[]" {
			if err := json.Unmarshal([]byte(exercisesJSON), &w.Exercises); err != nil {
				slog.Warn("failed to unmarshal exercises_json", "err", err)
			}
		}
		if metadataJSON != "" && metadataJSON != "{}" {
			if err := json.Unmarshal([]byte(metadataJSON), &w.Metadata); err != nil {
				slog.Warn("failed to unmarshal metadata_json", "err", err)
			}
		}
		out = append(out, w)
	}
	return out, nil
}

// TargetStore
func (s *SQLiteStore) FetchTargets(ctx context.Context, userID string) (models.Targets, error) {
	var t models.Targets
	var eatBack int
	err := s.db.QueryRowContext(ctx, `SELECT user_id, calories, protein_g, carbs_g, fat_g, fiber_g, water_ml, COALESCE(eat_back_exercise,0), updated_at FROM targets WHERE user_id=?`, userID).Scan(&t.UserID, &t.Calories, &t.ProteinG, &t.CarbsG, &t.FatG, &t.FiberG, &t.WaterMl, &eatBack, &t.UpdatedAt)
	if err != nil {
		return t, err
	}
	t.EatBackExercise = eatBack == 1
	return t, nil
}

// DB exposes the underlying *sql.DB for callers that need to run custom queries.
func (s *SQLiteStore) DB() *sql.DB { return s.db }
