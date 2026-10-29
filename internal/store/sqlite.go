package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/dfgoodfellow2/diet-tracker/v2/internal/models"
	"github.com/dfgoodfellow2/diet-tracker/v2/internal/services/metrics"
)

// NewSQLiteStore wraps *sql.DB to implement Store interface.
type SQLiteStore struct{ db *sql.DB }

func NewSQLiteStore(db *sql.DB) *SQLiteStore { return &SQLiteStore{db: db} }

// Ensure interface
var _ Store = (*SQLiteStore)(nil)

// Note: DB() accessor removed — use PingContext and store methods instead.

// UserStore
func (s *SQLiteStore) FetchProfile(ctx context.Context, userID string) (models.Profile, error) {
	var p models.Profile
	err := s.db.QueryRowContext(ctx, `
        SELECT user_id, COALESCE(name,''), COALESCE(age,0), COALESCE(sex,''),
               COALESCE(height_cm,0), COALESCE(activity,'sedentary'), COALESCE(exercise_freq,0),
               COALESCE(running_km,0), COALESCE(is_lifter,0), COALESCE(goal,'maintenance'),
               COALESCE(prioritize_carbs,0), COALESCE(bf_pct,0), COALESCE(hr_rest,0),
               COALESCE(hr_max,0), COALESCE(grip_weight,0.5), COALESCE(tdee_lookback_days,90),
               COALESCE(sleep_quality_max,10.0), COALESCE(units,'imperial'), updated_at,
               COALESCE(birthdate,''), COALESCE(macro_distribution,'balanced')
        FROM profiles WHERE user_id = ?`, userID,
	).Scan(&p.UserID, &p.Name, &p.Age, &p.Sex, &p.HeightCm, &p.Activity, &p.ExerciseFreq,
		&p.RunningKm, &p.IsLifter, &p.Goal, &p.PrioritizeCarbs, &p.BfPct, &p.HRRest,
		&p.HRMax, &p.GripWeight, &p.TDEELookbackDays, &p.SleepQualityMax, &p.Units, &p.UpdatedAt,
		&p.Birthdate, &p.MacroDistribution)
	if err != nil {
		slog.Error("database operation failed", "err", err, "operation", "fetch_profile", "user_id", userID)
	}
	return p, err
}

// UpsertProfile inserts or updates a profile row.
func (s *SQLiteStore) UpsertProfile(ctx context.Context, p *models.Profile) error {
	isLifter := 0
	if p.IsLifter {
		isLifter = 1
	}
	prioritize := 0
	if p.PrioritizeCarbs {
		prioritize = 1
	}
	_, err := s.db.ExecContext(ctx, `
        INSERT INTO profiles (user_id,name,age,sex,height_cm,activity,exercise_freq,running_km,
          is_lifter,goal,prioritize_carbs,bf_pct,hr_rest,hr_max,grip_weight,tdee_lookback_days,
          sleep_quality_max,units,updated_at,birthdate,macro_distribution)
        VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
        ON CONFLICT(user_id) DO UPDATE SET
          name=excluded.name, age=excluded.age, sex=excluded.sex, height_cm=excluded.height_cm,
          activity=excluded.activity, exercise_freq=excluded.exercise_freq,
          running_km=excluded.running_km, is_lifter=excluded.is_lifter, goal=excluded.goal,
          prioritize_carbs=excluded.prioritize_carbs, bf_pct=excluded.bf_pct, hr_rest=excluded.hr_rest,
          hr_max=excluded.hr_max, grip_weight=excluded.grip_weight,
          tdee_lookback_days=excluded.tdee_lookback_days, sleep_quality_max=excluded.sleep_quality_max,
          units=excluded.units, updated_at=excluded.updated_at,
          birthdate=excluded.birthdate, macro_distribution=excluded.macro_distribution`,
		p.UserID, p.Name, p.Age, p.Sex, p.HeightCm, p.Activity, p.ExerciseFreq, p.RunningKm,
		isLifter, p.Goal, prioritize, p.BfPct, p.HRRest, p.HRMax, p.GripWeight,
		p.TDEELookbackDays, p.SleepQualityMax, p.Units, p.UpdatedAt, p.Birthdate, p.MacroDistribution,
	)
	if err != nil {
		slog.Error("database operation failed", "err", err, "operation", "upsert_profile", "user_id", p.UserID)
	}
	return err
}

// LogStore
func (s *SQLiteStore) FetchNutritionLogs(ctx context.Context, userID, since string) ([]models.NutritionLog, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, user_id, date, calories, protein_g, carbs_g, fat_g, fiber_g, water_ml, COALESCE(meal_notes,''), updated_at FROM nutrition_logs WHERE user_id=? AND date >= ? ORDER BY date ASC`, userID, since)
	if err != nil {
		return nil, fmt.Errorf("fetch nutrition logs: %w", err)
	}
	defer rows.Close()
	var out []models.NutritionLog
	for rows.Next() {
		var n models.NutritionLog
		if err := rows.Scan(&n.ID, &n.UserID, &n.Date, &n.Calories, &n.ProteinG, &n.CarbsG, &n.FatG, &n.FiberG, &n.WaterMl, &n.MealNotes, &n.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan nutrition log row: %w", err)
		}
		out = append(out, n)
	}
	return out, nil
}

func (s *SQLiteStore) FetchBiometricLogs(ctx context.Context, userID, since string) ([]models.BiometricLog, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, user_id, date, COALESCE(weight_kg,0), COALESCE(waist_cm,0), COALESCE(grip_kg,0), COALESCE(bolt_score,0), COALESCE(sleep_hours,0), COALESCE(sleep_quality,0), COALESCE(subjective_feel,0), COALESCE(body_fat_pct,0), COALESCE(notes,''), updated_at FROM biometric_logs WHERE user_id=? AND date >= ? ORDER BY date ASC`, userID, since)
	if err != nil {
		return nil, fmt.Errorf("fetch biometric logs: %w", err)
	}
	defer rows.Close()
	var out []models.BiometricLog
	for rows.Next() {
		var b models.BiometricLog
		if err := rows.Scan(&b.ID, &b.UserID, &b.Date, &b.WeightKg, &b.WaistCm, &b.GripKg, &b.BoltScore, &b.SleepHours, &b.SleepQuality, &b.SubjectiveFeel, &b.BodyFatPct, &b.Notes, &b.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan biometric log row: %w", err)
		}
		out = append(out, b)
	}
	return out, nil
}

func (s *SQLiteStore) FetchLatestWeight(ctx context.Context, userID string) (float64, error) {
	var w float64
	err := s.db.QueryRowContext(ctx, `SELECT weight_kg FROM biometric_logs WHERE user_id=? AND weight_kg > 0 ORDER BY date DESC LIMIT 1`, userID).Scan(&w)
	if err != nil {
		return 0, fmt.Errorf("fetch latest weight: %w", err)
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
		return 0, 0, 0, fmt.Errorf("fetch body measurements: %w", err)
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
		return "", fmt.Errorf("fetch last checkin: %w", err)
	}
	return lastCheckinDate, nil
}

func (s *SQLiteStore) GetLastCheckin(ctx context.Context, userID string) (models.CheckInLog, error) {
	var out models.CheckInLog
	err := s.db.QueryRowContext(ctx, `SELECT id, user_id, check_in_date, weight_before, weight_after, calories_before, calories_after, reason, created_at, observed_tdee, tdee_confidence, lookback_days FROM check_in_logs WHERE user_id = ? ORDER BY check_in_date DESC LIMIT 1`, userID).Scan(&out.ID, &out.UserID, &out.CheckInDate, &out.WeightBefore, &out.WeightAfter, &out.CaloriesBefore, &out.CaloriesAfter, &out.Reason, &out.CreatedAt, &out.ObservedTDEE, &out.TDEEConfidence, &out.LookbackDays)
	if err != nil {
		return out, fmt.Errorf("get last checkin: %w", err)
	}
	return out, nil
}

// CreateCheckinLog inserts a new check-in row.
func (s *SQLiteStore) CreateCheckinLog(ctx context.Context, c *models.CheckInLog) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO check_in_logs (id, user_id, check_in_date, weight_before, weight_after, calories_before, calories_after, reason, created_at, observed_tdee, tdee_confidence, lookback_days) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`, c.ID, c.UserID, c.CheckInDate, c.WeightBefore, c.WeightAfter, c.CaloriesBefore, c.CaloriesAfter, c.Reason, c.CreatedAt, c.ObservedTDEE, c.TDEEConfidence, c.LookbackDays)
	if err != nil {
		return fmt.Errorf("create checkin log: %w", err)
	}
	return nil
}

// FetchCheckinHistory returns the most recent check-ins for a user, newest
// first. ResultingCalories is the post-check-in calorie target that was
// persisted for each event.
func (s *SQLiteStore) FetchCheckinHistory(ctx context.Context, userID string, limit int) ([]models.CheckinHistoryEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `SELECT check_in_date, observed_tdee, tdee_confidence, lookback_days, COALESCE(calories_after,0) FROM check_in_logs WHERE user_id = ? ORDER BY check_in_date DESC LIMIT ?`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch checkin history: %w", err)
	}
	defer rows.Close()
	var out []models.CheckinHistoryEntry
	for rows.Next() {
		var e models.CheckinHistoryEntry
		if err := rows.Scan(&e.CheckInDate, &e.ObservedTDEE, &e.TDEEConfidence, &e.LookbackDays, &e.ResultingCalories); err != nil {
			return nil, fmt.Errorf("scan checkin history row: %w", err)
		}
		out = append(out, e)
	}
	return out, nil
}

// FindUserByUsername returns a user by username.
func (s *SQLiteStore) FindUserByUsername(ctx context.Context, username string) (models.User, error) {
	var u models.User
	var isAdmin int
	err := s.db.QueryRowContext(ctx, `SELECT id, username, email, is_admin, created_at, updated_at FROM users WHERE username = ? LIMIT 1`, username).Scan(&u.ID, &u.Username, &u.Email, &isAdmin, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return u, fmt.Errorf("find user by username: %w", err)
	}
	u.IsAdmin = isAdmin == 1
	return u, nil
}

// FindUserByID is an alias to GetUserByID kept for compatibility.
func (s *SQLiteStore) FindUserByID(ctx context.Context, userID string) (models.User, error) {
	return s.GetUserByID(ctx, userID)
}

// FetchGripPB returns the maximum grip_kg for the user in the 120 days before `today`.
func (s *SQLiteStore) FetchGripPB(ctx context.Context, userID, today string) (float64, error) {
	var gripPB float64
	row := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(grip_kg),0) FROM biometric_logs WHERE user_id = ? AND grip_kg > 0 AND date >= DATE(?, '-120 days')`, userID, today)
	if err := row.Scan(&gripPB); err != nil {
		return 0, fmt.Errorf("fetch grip pb: %w", err)
	}
	return gripPB, nil
}

// WorkoutStore
func (s *SQLiteStore) FetchWorkouts(ctx context.Context, userID, since string) ([]models.WorkoutEntry, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, user_id, date, slot, COALESCE(title,''), COALESCE(raw_notes,''), COALESCE(duration_min,0), COALESCE(calories_burned,0), COALESCE(mwv,0), COALESCE(nds,0), COALESCE(session_density,0), COALESCE(exercises_json,'[]'), COALESCE(metadata_json,'{}'), updated_at FROM workouts WHERE user_id=? AND date >= ? ORDER BY date ASC`, userID, since)
	if err != nil {
		return nil, fmt.Errorf("fetch workouts: %w", err)
	}
	defer rows.Close()
	var out []models.WorkoutEntry
	for rows.Next() {
		var w models.WorkoutEntry
		var exercisesJSON, metadataJSON string
		if err := rows.Scan(&w.ID, &w.UserID, &w.Date, &w.Slot, &w.Title, &w.RawNotes, &w.DurationMin, &w.CaloriesBurned, &w.MWV, &w.NDS, &w.SessionDensity, &exercisesJSON, &metadataJSON, &w.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan workout row: %w", err)
		}
		if exercisesJSON != "" && exercisesJSON != "[]" {
			if err := json.Unmarshal([]byte(exercisesJSON), &w.Exercises); err != nil {
				slog.Warn("failed to unmarshal exercises_json", "err", err)
			} else {
				// Backwards-compat: some DB rows used snake_case `duration_raw`.
				// If the new camelCase DurationRaw is empty, populate it from
				// the legacy field so API responses include durationRaw.
				for i := range w.Exercises {
					if w.Exercises[i].DurationRaw == "" && w.Exercises[i].DurationRawLegacy != "" {
						w.Exercises[i].DurationRaw = w.Exercises[i].DurationRawLegacy
					}
				}
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
		return t, fmt.Errorf("fetch targets: %w", err)
	}
	t.EatBackExercise = eatBack == 1
	return t, nil
}

// PingContext implements Store.PingContext
func (s *SQLiteStore) PingContext(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// CountCheckInLogs removed — no longer required by store interface.

// RawQueryTables removed — temporary debug helper no longer needed.

// --- UserStore implementations ---
func (s *SQLiteStore) CountUsers(ctx context.Context) (int, error) {
	var cnt int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&cnt); err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return cnt, nil
}

func (s *SQLiteStore) CreateUser(ctx context.Context, id, username, email, passwordHash string, isAdmin bool, createdAt, updatedAt string) error {
	admin := 0
	if isAdmin {
		admin = 1
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO users (id, username, email, password, is_admin, created_at, updated_at) VALUES (?,?,?,?,?,?,?)`, id, username, email, passwordHash, admin, createdAt, updatedAt)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (s *SQLiteStore) FindUserByLogin(ctx context.Context, login string) (id, username, passwordHash string, isAdmin bool, err error) {
	var adminInt int
	err = s.db.QueryRowContext(ctx, `SELECT id, username, password, is_admin FROM users WHERE username = ? OR email = ? LIMIT 1`, login, login).Scan(&id, &username, &passwordHash, &adminInt)
	if err != nil {
		return "", "", "", false, fmt.Errorf("find user by login: %w", err)
	}
	return id, username, passwordHash, adminInt == 1, nil
}

func (s *SQLiteStore) GetUserByID(ctx context.Context, userID string) (models.User, error) {
	var u models.User
	var isAdmin int
	if err := s.db.QueryRowContext(ctx, `SELECT id, username, email, is_admin, created_at, updated_at FROM users WHERE id = ?`, userID).Scan(&u.ID, &u.Username, &u.Email, &isAdmin, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return u, fmt.Errorf("get user by id: %w", err)
	}
	u.IsAdmin = isAdmin == 1
	return u, nil
}

func (s *SQLiteStore) SaveRefreshToken(ctx context.Context, id, userID, tokenHash, expiresAt, createdAt string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, created_at) VALUES (?,?,?,?,?)`, id, userID, tokenHash, expiresAt, createdAt)
	if err != nil {
		return fmt.Errorf("save refresh token: %w", err)
	}
	return nil
}

func (s *SQLiteStore) FindRefreshToken(ctx context.Context, tokenHash string) (userID string, isAdmin bool, expiresAt string, err error) {
	var adminInt int
	err = s.db.QueryRowContext(ctx, `SELECT u.id, u.is_admin, rt.expires_at FROM refresh_tokens rt JOIN users u ON u.id = rt.user_id WHERE rt.token_hash = ?`, tokenHash).Scan(&userID, &adminInt, &expiresAt)
	if err != nil {
		return "", false, "", fmt.Errorf("find refresh token: %w", err)
	}
	return userID, adminInt == 1, expiresAt, nil
}

func (s *SQLiteStore) DeleteRefreshToken(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE token_hash = ?`, tokenHash)
	if err != nil {
		return fmt.Errorf("delete refresh token: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ListUsers(ctx context.Context) ([]models.User, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, username, email, is_admin, created_at, updated_at FROM users ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	var out []models.User
	for rows.Next() {
		var u models.User
		var isAdmin int
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &isAdmin, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan user row: %w", err)
		}
		u.IsAdmin = isAdmin == 1
		out = append(out, u)
	}
	return out, nil
}

func (s *SQLiteStore) DeleteUser(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, userID)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

func (s *SQLiteStore) PromoteUser(ctx context.Context, userID, now string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET is_admin = 1, updated_at = ? WHERE id = ?`, now, userID)
	if err != nil {
		return fmt.Errorf("promote user: %w", err)
	}
	return nil
}

func (s *SQLiteStore) DemoteUser(ctx context.Context, userID, now string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET is_admin = 0, updated_at = ? WHERE id = ?`, now, userID)
	if err != nil {
		return fmt.Errorf("demote user: %w", err)
	}
	return nil
}

func (s *SQLiteStore) CountAdmins(ctx context.Context) (int, error) {
	var cnt int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE is_admin = 1`).Scan(&cnt); err != nil {
		return 0, fmt.Errorf("count admins: %w", err)
	}
	return cnt, nil
}

// --- Nutrition ---
func (s *SQLiteStore) FetchNutritionLogsRange(ctx context.Context, userID, from, to string) ([]models.NutritionLog, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,user_id,date,calories,protein_g,carbs_g,fat_g,fiber_g,water_ml,meal_notes,updated_at FROM nutrition_logs WHERE user_id = ? AND date >= ? AND date <= ? ORDER BY date ASC`, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("fetch nutrition logs range: %w", err)
	}
	defer rows.Close()
	var out []models.NutritionLog
	for rows.Next() {
		var n models.NutritionLog
		if err := rows.Scan(&n.ID, &n.UserID, &n.Date, &n.Calories, &n.ProteinG, &n.CarbsG, &n.FatG, &n.FiberG, &n.WaterMl, &n.MealNotes, &n.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan nutrition log range row: %w", err)
		}
		out = append(out, n)
	}
	return out, nil
}

func (s *SQLiteStore) GetNutritionLog(ctx context.Context, userID, date string) (models.NutritionLog, error) {
	var n models.NutritionLog
	err := s.db.QueryRowContext(ctx, `SELECT id,user_id,date,calories,protein_g,carbs_g,fat_g,fiber_g,water_ml,meal_notes,updated_at FROM nutrition_logs WHERE user_id = ? AND date = ?`, userID, date).Scan(&n.ID, &n.UserID, &n.Date, &n.Calories, &n.ProteinG, &n.CarbsG, &n.FatG, &n.FiberG, &n.WaterMl, &n.MealNotes, &n.UpdatedAt)
	if err != nil {
		return n, fmt.Errorf("get nutrition log: %w", err)
	}
	return n, nil
}

func (s *SQLiteStore) CreateNutritionLog(ctx context.Context, n *models.NutritionLog) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO nutrition_logs (id,user_id,date,calories,protein_g,carbs_g,fat_g,fiber_g,water_ml,meal_notes,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)`, n.ID, n.UserID, n.Date, n.Calories, n.ProteinG, n.CarbsG, n.FatG, n.FiberG, n.WaterMl, n.MealNotes, n.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create nutrition log: %w", err)
	}
	return nil
}

func (s *SQLiteStore) UpdateNutritionLog(ctx context.Context, n *models.NutritionLog) error {
	_, err := s.db.ExecContext(ctx, `UPDATE nutrition_logs SET calories = ?, protein_g = ?, carbs_g = ?, fat_g = ?, fiber_g = ?, water_ml = ?, meal_notes = ?, updated_at = ? WHERE user_id = ? AND date = ?`, n.Calories, n.ProteinG, n.CarbsG, n.FatG, n.FiberG, n.WaterMl, n.MealNotes, n.UpdatedAt, n.UserID, n.Date)
	if err != nil {
		return fmt.Errorf("update nutrition log: %w", err)
	}
	return nil
}

func (s *SQLiteStore) DeleteNutritionLog(ctx context.Context, userID, date string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM nutrition_logs WHERE user_id = ? AND date = ?`, userID, date)
	if err != nil {
		return fmt.Errorf("delete nutrition log: %w", err)
	}
	return nil
}

// --- Biometric ---
func (s *SQLiteStore) FetchBiometricLogsRange(ctx context.Context, userID, from, to string) ([]models.BiometricLog, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,user_id,date,weight_kg,waist_cm,grip_kg,bolt_score,sleep_hours,sleep_quality,subjective_feel,body_fat_pct,notes,updated_at FROM biometric_logs WHERE user_id = ? AND date >= ? AND date <= ? ORDER BY date ASC`, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("fetch biometric logs range: %w", err)
	}
	defer rows.Close()
	var out []models.BiometricLog
	for rows.Next() {
		var b models.BiometricLog
		if err := rows.Scan(&b.ID, &b.UserID, &b.Date, &b.WeightKg, &b.WaistCm, &b.GripKg, &b.BoltScore, &b.SleepHours, &b.SleepQuality, &b.SubjectiveFeel, &b.BodyFatPct, &b.Notes, &b.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan biometric log range row: %w", err)
		}
		out = append(out, b)
	}
	return out, nil
}

func (s *SQLiteStore) GetBiometricLog(ctx context.Context, userID, date string) (models.BiometricLog, error) {
	var b models.BiometricLog
	err := s.db.QueryRowContext(ctx, `SELECT id,user_id,date,weight_kg,waist_cm,grip_kg,bolt_score,sleep_hours,sleep_quality,subjective_feel,body_fat_pct,notes,updated_at FROM biometric_logs WHERE user_id = ? AND date = ?`, userID, date).Scan(&b.ID, &b.UserID, &b.Date, &b.WeightKg, &b.WaistCm, &b.GripKg, &b.BoltScore, &b.SleepHours, &b.SleepQuality, &b.SubjectiveFeel, &b.BodyFatPct, &b.Notes, &b.UpdatedAt)
	if err != nil {
		return b, fmt.Errorf("get biometric log: %w", err)
	}
	return b, nil
}

func (s *SQLiteStore) CreateBiometricLog(ctx context.Context, b *models.BiometricLog) error {
	_, err := s.db.ExecContext(ctx, `
        INSERT INTO biometric_logs (id,user_id,date,weight_kg,waist_cm,grip_kg,bolt_score,sleep_hours,sleep_quality,subjective_feel,body_fat_pct,notes,updated_at)
        VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)
        ON CONFLICT(user_id,date) DO UPDATE SET
          weight_kg      = CASE WHEN excluded.weight_kg      != 0   THEN excluded.weight_kg      ELSE weight_kg      END,
          waist_cm       = CASE WHEN excluded.waist_cm       != 0   THEN excluded.waist_cm       ELSE waist_cm       END,
          grip_kg        = CASE WHEN excluded.grip_kg        != 0   THEN excluded.grip_kg        ELSE grip_kg        END,
          bolt_score     = CASE WHEN excluded.bolt_score     != 0   THEN excluded.bolt_score     ELSE bolt_score     END,
          sleep_hours    = CASE WHEN excluded.sleep_hours    != 0   THEN excluded.sleep_hours    ELSE sleep_hours    END,
          sleep_quality  = CASE WHEN excluded.sleep_quality  != 0   THEN excluded.sleep_quality  ELSE sleep_quality  END,
          subjective_feel= CASE WHEN excluded.subjective_feel!= 0   THEN excluded.subjective_feel ELSE subjective_feel END,
          body_fat_pct   = CASE WHEN excluded.body_fat_pct   != 0   THEN excluded.body_fat_pct   ELSE body_fat_pct   END,
          notes          = CASE WHEN excluded.notes          != ''  THEN excluded.notes          ELSE notes          END,
          updated_at     = excluded.updated_at`,
		b.ID, b.UserID, b.Date, b.WeightKg, b.WaistCm, b.GripKg, b.BoltScore,
		b.SleepHours, b.SleepQuality, b.SubjectiveFeel, b.BodyFatPct, b.Notes, b.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create biometric log: %w", err)
	}
	return nil
}

func (s *SQLiteStore) UpdateBiometricLog(ctx context.Context, b *models.BiometricLog) error {
	_, err := s.db.ExecContext(ctx, `UPDATE biometric_logs SET weight_kg=?,waist_cm=?,grip_kg=?,bolt_score=?,sleep_hours=?,sleep_quality=?,subjective_feel=?,body_fat_pct=?,notes=?,updated_at=? WHERE user_id=? AND date = ?`, b.WeightKg, b.WaistCm, b.GripKg, b.BoltScore, b.SleepHours, b.SleepQuality, b.SubjectiveFeel, b.BodyFatPct, b.Notes, b.UpdatedAt, b.UserID, b.Date)
	if err != nil {
		return fmt.Errorf("update biometric log: %w", err)
	}
	return nil
}

func (s *SQLiteStore) DeleteBiometricLog(ctx context.Context, userID, date string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM biometric_logs WHERE user_id = ? AND date = ?`, userID, date)
	if err != nil {
		return fmt.Errorf("delete biometric log: %w", err)
	}
	return nil
}

// --- Workouts ---
func (s *SQLiteStore) FetchWorkoutsRange(ctx context.Context, userID, from, to string) ([]models.WorkoutEntry, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, user_id, date, slot, COALESCE(title,''), COALESCE(raw_notes,''), COALESCE(duration_min,0), COALESCE(calories_burned,0), COALESCE(mwv,0), COALESCE(nds,0), COALESCE(session_density,0), COALESCE(exercises_json,'[]'), COALESCE(metadata_json,'{}'), updated_at FROM workouts WHERE user_id = ? AND date >= ? AND date <= ? ORDER BY date ASC, slot ASC`, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("fetch workouts range: %w", err)
	}
	defer rows.Close()
	var out []models.WorkoutEntry
	for rows.Next() {
		var w models.WorkoutEntry
		var exercisesJSON, metadataJSON string
		if err := rows.Scan(&w.ID, &w.UserID, &w.Date, &w.Slot, &w.Title, &w.RawNotes, &w.DurationMin, &w.CaloriesBurned, &w.MWV, &w.NDS, &w.SessionDensity, &exercisesJSON, &metadataJSON, &w.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan workout range row: %w", err)
		}
		if exercisesJSON != "" && exercisesJSON != "[]" {
			if err := json.Unmarshal([]byte(exercisesJSON), &w.Exercises); err != nil {
				slog.Warn("failed to unmarshal exercises_json", "err", err)
			} else {
				for i := range w.Exercises {
					if w.Exercises[i].DurationRaw == "" && w.Exercises[i].DurationRawLegacy != "" {
						w.Exercises[i].DurationRaw = w.Exercises[i].DurationRawLegacy
					}
				}
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

func (s *SQLiteStore) GetWorkout(ctx context.Context, userID, date, slot string) (models.WorkoutEntry, error) {
	var w models.WorkoutEntry
	var exercisesJSON, metadataJSON string
	err := s.db.QueryRowContext(ctx, `SELECT id, user_id, date, slot, COALESCE(title,''), COALESCE(raw_notes,''), COALESCE(duration_min,0), COALESCE(calories_burned,0), COALESCE(mwv,0), COALESCE(nds,0), COALESCE(session_density,0), COALESCE(exercises_json,'[]'), COALESCE(metadata_json,'{}'), updated_at FROM workouts WHERE user_id = ? AND date = ? AND slot = ?`, userID, date, slot).Scan(&w.ID, &w.UserID, &w.Date, &w.Slot, &w.Title, &w.RawNotes, &w.DurationMin, &w.CaloriesBurned, &w.MWV, &w.NDS, &w.SessionDensity, &exercisesJSON, &metadataJSON, &w.UpdatedAt)
	if err != nil {
		return w, fmt.Errorf("get workout: %w", err)
	}
	if exercisesJSON != "" {
		if err := json.Unmarshal([]byte(exercisesJSON), &w.Exercises); err != nil {
			slog.Warn("failed to unmarshal exercises_json", "err", err)
		} else {
			for i := range w.Exercises {
				if w.Exercises[i].DurationRaw == "" && w.Exercises[i].DurationRawLegacy != "" {
					w.Exercises[i].DurationRaw = w.Exercises[i].DurationRawLegacy
				}
			}
		}
	}
	if metadataJSON != "" {
		_ = json.Unmarshal([]byte(metadataJSON), &w.Metadata)
	}
	return w, nil
}

func (s *SQLiteStore) UpsertWorkout(ctx context.Context, w *models.WorkoutEntry) error {
	exb, _ := json.Marshal(w.Exercises)
	metb, _ := json.Marshal(w.Metadata)
	// Try update first
	res, err := s.db.ExecContext(ctx, `UPDATE workouts SET title=?, raw_notes=?, duration_min=?, calories_burned=?, exercises_json=?, metadata_json=?, updated_at=?, mwv=?, nds=?, session_density=? WHERE user_id=? AND date=? AND slot=?`, w.Title, w.RawNotes, w.DurationMin, w.CaloriesBurned, string(exb), string(metb), w.UpdatedAt, w.MWV, w.NDS, w.SessionDensity, w.UserID, w.Date, w.Slot)
	if err != nil {
		return fmt.Errorf("upsert workout update: %w", err)
	}
	if ra, _ := res.RowsAffected(); ra == 0 {
		_, err = s.db.ExecContext(ctx, `INSERT INTO workouts (id,user_id,date,slot,title,raw_notes,duration_min,calories_burned,mwv,nds,session_density,exercises_json,metadata_json,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, w.ID, w.UserID, w.Date, w.Slot, w.Title, w.RawNotes, w.DurationMin, w.CaloriesBurned, w.MWV, w.NDS, w.SessionDensity, string(exb), string(metb), w.UpdatedAt)
		if err != nil {
			return fmt.Errorf("upsert workout insert: %w", err)
		}
	}
	return nil
}

func (s *SQLiteStore) UpdateWorkout(ctx context.Context, w *models.WorkoutEntry) (models.WorkoutEntry, error) {
	exb, _ := json.Marshal(w.Exercises)
	metb, _ := json.Marshal(w.Metadata)
	_, err := s.db.ExecContext(ctx, `UPDATE workouts SET title=?,raw_notes=?,duration_min=?,calories_burned=?,mwv=?,nds=?,session_density=?,exercises_json=?,metadata_json=?,updated_at=? WHERE user_id=? AND date=? AND slot=?`, w.Title, w.RawNotes, w.DurationMin, w.CaloriesBurned, w.MWV, w.NDS, w.SessionDensity, string(exb), string(metb), w.UpdatedAt, w.UserID, w.Date, w.Slot)
	if err != nil {
		return models.WorkoutEntry{}, fmt.Errorf("update workout: %w", err)
	}
	out, err := s.GetWorkout(ctx, w.UserID, w.Date, w.Slot)
	if err != nil {
		return out, fmt.Errorf("update workout get: %w", err)
	}
	return out, nil
}

func (s *SQLiteStore) DeleteWorkout(ctx context.Context, userID, date, slot string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM workouts WHERE user_id = ? AND date = ? AND slot = ?`, userID, date, slot)
	if err != nil {
		return fmt.Errorf("delete workout: %w", err)
	}
	return nil
}

// --- Targets ---
func (s *SQLiteStore) UpsertTargets(ctx context.Context, t *models.Targets) error {
	eatBack := 0
	if t.EatBackExercise {
		eatBack = 1
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO targets (user_id,calories,protein_g,carbs_g,fat_g,fiber_g,water_ml,eat_back_exercise,updated_at) VALUES (?,?,?,?,?,?,?,?,?) ON CONFLICT(user_id) DO UPDATE SET calories=excluded.calories,protein_g=excluded.protein_g,carbs_g=excluded.carbs_g,fat_g=excluded.fat_g,fiber_g=excluded.fiber_g,water_ml=excluded.water_ml,eat_back_exercise=excluded.eat_back_exercise,updated_at=excluded.updated_at`, t.UserID, t.Calories, t.ProteinG, t.CarbsG, t.FatG, t.FiberG, t.WaterMl, eatBack, t.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert targets: %w", err)
	}
	return nil
}

func (s *SQLiteStore) CreateTargetSnapshot(ctx context.Context, snap *models.TargetSnapshot) error {
	eatBack := 0
	if snap.EatBackExercise {
		eatBack = 1
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO target_history (id,user_id,effective_date,calories,protein_g,carbs_g,fat_g,fiber_g,water_ml,eat_back_exercise,created_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)`, snap.ID, snap.UserID, snap.EffectiveDate, snap.Calories, snap.ProteinG, snap.CarbsG, snap.FatG, snap.FiberG, snap.WaterMl, eatBack, snap.CreatedAt)
	if err != nil {
		return fmt.Errorf("create target snapshot: %w", err)
	}
	return nil
}

func (s *SQLiteStore) FetchTargetHistory(ctx context.Context, userID string) ([]models.TargetSnapshot, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,user_id,effective_date,calories,protein_g,carbs_g,fat_g,fiber_g,water_ml,eat_back_exercise,created_at FROM target_history WHERE user_id = ? ORDER BY effective_date DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("fetch target history: %w", err)
	}
	defer rows.Close()
	var out []models.TargetSnapshot
	for rows.Next() {
		var t models.TargetSnapshot
		var eatBack int
		if err := rows.Scan(&t.ID, &t.UserID, &t.EffectiveDate, &t.Calories, &t.ProteinG, &t.CarbsG, &t.FatG, &t.FiberG, &t.WaterMl, &eatBack, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan target history row: %w", err)
		}
		t.EatBackExercise = eatBack == 1
		out = append(out, t)
	}
	return out, nil
}

// --- Meals ---
func (s *SQLiteStore) FetchSavedMeals(ctx context.Context, userID string) ([]models.SavedMeal, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,user_id,name,calories,protein_g,carbs_g,fat_g,fiber_g,notes,created_at,updated_at FROM saved_meals WHERE user_id = ? ORDER BY name ASC`, userID)
	if err != nil {
		return nil, fmt.Errorf("fetch saved meals: %w", err)
	}
	defer rows.Close()
	var out []models.SavedMeal
	for rows.Next() {
		var m models.SavedMeal
		if err := rows.Scan(&m.ID, &m.UserID, &m.Name, &m.Calories, &m.ProteinG, &m.CarbsG, &m.FatG, &m.FiberG, &m.Notes, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan saved meal row: %w", err)
		}
		out = append(out, m)
	}
	return out, nil
}

func (s *SQLiteStore) CreateSavedMeal(ctx context.Context, m *models.SavedMeal) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO saved_meals (id,user_id,name,calories,protein_g,carbs_g,fat_g,fiber_g,notes,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)`, m.ID, m.UserID, m.Name, m.Calories, m.ProteinG, m.CarbsG, m.FatG, m.FiberG, m.Notes, m.CreatedAt, m.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create saved meal: %w", err)
	}
	return nil
}

func (s *SQLiteStore) DeleteSavedMeal(ctx context.Context, userID, mealID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM saved_meals WHERE id = ? AND user_id = ?`, mealID, userID)
	if err != nil {
		return fmt.Errorf("delete saved meal: %w", err)
	}
	return nil
}

func (s *SQLiteStore) FetchMealTemplates(ctx context.Context, userID string) ([]models.MealTemplate, error) {
	// Own templates plus templates shared to any group the caller belongs to.
	// Shared rows carry the owner's profile name for the provenance badge.
	rows, err := s.db.QueryContext(ctx, `
        SELECT mt.id, mt.user_id, mt.name, mt.meals_json, mt.created_at, mt.updated_at,
               mt.group_id,
               COALESCE((SELECT NULLIF(p.name,'') FROM profiles p WHERE p.user_id = mt.user_id), '')
        FROM meal_templates mt
        WHERE mt.user_id = ?
           OR (mt.group_id <> '' AND mt.group_id IN (
                   SELECT gm.group_id FROM group_members gm WHERE gm.user_id = ?
               ))
        ORDER BY mt.name ASC`, userID, userID)
	if err != nil {
		return nil, fmt.Errorf("fetch meal templates: %w", err)
	}
	defer rows.Close()
	var out []models.MealTemplate
	for rows.Next() {
		var id, uid, name, mealsJSON, createdAt, updatedAt, groupID, ownerName string
		if err := rows.Scan(&id, &uid, &name, &mealsJSON, &createdAt, &updatedAt, &groupID, &ownerName); err != nil {
			return nil, fmt.Errorf("scan meal template row: %w", err)
		}
		var meals []models.SavedMeal
		if mealsJSON != "" {
			if err := json.Unmarshal([]byte(mealsJSON), &meals); err != nil {
				slog.Warn("unmarshal meals_json failed", "err", err)
			}
		}
		sharedBy := ""
		if uid != userID {
			sharedBy = ownerName
			if sharedBy == "" {
				sharedBy = "Member"
			}
		}
		out = append(out, models.MealTemplate{
			ID:        id,
			UserID:    uid,
			Name:      name,
			Meals:     meals,
			GroupID:   groupID,
			SharedBy:  sharedBy,
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		})
	}
	return out, nil
}

func (s *SQLiteStore) CreateMealTemplate(ctx context.Context, t *models.MealTemplate) error {
	b, _ := json.Marshal(t.Meals)
	_, err := s.db.ExecContext(ctx, `INSERT INTO meal_templates (id,user_id,name,meals_json,created_at,updated_at) VALUES (?,?,?,?,?,?)`, t.ID, t.UserID, t.Name, string(b), t.CreatedAt, t.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create meal template: %w", err)
	}
	return nil
}

func (s *SQLiteStore) DeleteMealTemplate(ctx context.Context, userID, templateID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM meal_templates WHERE id = ? AND user_id = ?`, templateID, userID)
	if err != nil {
		return fmt.Errorf("delete meal template: %w", err)
	}
	return nil
}

// --- Measurements ---
func (s *SQLiteStore) FetchMeasurementsRange(ctx context.Context, userID, from, to string) ([]models.BodyMeasurement, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,user_id,date,neck_cm,chest_cm,waist_cm,hips_cm,thigh_cm,bicep_cm,notes,created_at,shoulders_cm,calves_cm FROM body_measurements WHERE user_id = ? AND date >= ? AND date <= ? ORDER BY date ASC`, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("fetch measurements range: %w", err)
	}
	defer rows.Close()
	var out []models.BodyMeasurement
	for rows.Next() {
		var m models.BodyMeasurement
		if err := rows.Scan(&m.ID, &m.UserID, &m.Date, &m.NeckCm, &m.ChestCm, &m.WaistCm, &m.HipsCm, &m.ThighCm, &m.BicepCm, &m.Notes, &m.CreatedAt, &m.ShouldersCm, &m.CalvesCm); err != nil {
			return nil, fmt.Errorf("scan measurement row: %w", err)
		}
		out = append(out, m)
	}
	return out, nil
}

func (s *SQLiteStore) CreateMeasurement(ctx context.Context, m *models.BodyMeasurement) error {
	// POST semantics mirror POST /biometrics: preserve any existing non-zero
	// value when the incoming field is zero. This prevents clients that do not
	// collect every circumference (e.g. the TUI, which omits shoulders/calves)
	// from wiping fields they never sent. PUT /measurements/{date} remains a
	// true replace for intentional clears.
	_, err := s.db.ExecContext(ctx, `
        INSERT INTO body_measurements (id,user_id,date,neck_cm,chest_cm,waist_cm,hips_cm,thigh_cm,bicep_cm,notes,created_at,shoulders_cm,calves_cm)
        VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)
        ON CONFLICT(user_id,date) DO UPDATE SET
          neck_cm      = CASE WHEN excluded.neck_cm      != 0   THEN excluded.neck_cm      ELSE neck_cm      END,
          chest_cm     = CASE WHEN excluded.chest_cm     != 0   THEN excluded.chest_cm     ELSE chest_cm     END,
          waist_cm     = CASE WHEN excluded.waist_cm     != 0   THEN excluded.waist_cm     ELSE waist_cm     END,
          hips_cm      = CASE WHEN excluded.hips_cm      != 0   THEN excluded.hips_cm      ELSE hips_cm      END,
          thigh_cm     = CASE WHEN excluded.thigh_cm     != 0   THEN excluded.thigh_cm     ELSE thigh_cm     END,
          bicep_cm     = CASE WHEN excluded.bicep_cm     != 0   THEN excluded.bicep_cm     ELSE bicep_cm     END,
          shoulders_cm = CASE WHEN excluded.shoulders_cm != 0   THEN excluded.shoulders_cm ELSE shoulders_cm END,
          calves_cm    = CASE WHEN excluded.calves_cm    != 0   THEN excluded.calves_cm    ELSE calves_cm    END,
          notes        = CASE WHEN excluded.notes        != ''  THEN excluded.notes        ELSE notes        END`,
		m.ID, m.UserID, m.Date, m.NeckCm, m.ChestCm, m.WaistCm, m.HipsCm, m.ThighCm, m.BicepCm, m.Notes, m.CreatedAt, m.ShouldersCm, m.CalvesCm)
	if err != nil {
		return fmt.Errorf("create measurement: %w", err)
	}
	return nil
}

func (s *SQLiteStore) UpdateMeasurement(ctx context.Context, m *models.BodyMeasurement) error {
	_, err := s.db.ExecContext(ctx, `UPDATE body_measurements SET neck_cm=?,chest_cm=?,waist_cm=?,hips_cm=?,thigh_cm=?,bicep_cm=?,notes=?,shoulders_cm=?,calves_cm=? WHERE user_id=? AND date=?`, m.NeckCm, m.ChestCm, m.WaistCm, m.HipsCm, m.ThighCm, m.BicepCm, m.Notes, m.ShouldersCm, m.CalvesCm, m.UserID, m.Date)
	if err != nil {
		return fmt.Errorf("update measurement: %w", err)
	}
	return nil
}

func (s *SQLiteStore) DeleteMeasurement(ctx context.Context, userID, date string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM body_measurements WHERE user_id = ? AND date = ?`, userID, date)
	if err != nil {
		return fmt.Errorf("delete measurement: %w", err)
	}
	return nil
}

func (s *SQLiteStore) FetchBodyMeasurementsRangeMap(ctx context.Context, userID, from, to string) (map[string]float64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT date, COALESCE(waist_cm,0) FROM body_measurements WHERE user_id = ? AND date >= ? AND date <= ?`, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("fetch body measurements range map: %w", err)
	}
	defer rows.Close()
	m := make(map[string]float64)
	for rows.Next() {
		var d string
		var waist float64
		if err := rows.Scan(&d, &waist); err == nil {
			if waist > 0 {
				m[d] = waist
			}
		}
	}
	return m, nil
}

// --- Groups ---

// CreateGroup inserts a group and its owner membership atomically.
func (s *SQLiteStore) CreateGroup(ctx context.Context, g *models.Group) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin create group: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO groups (id, name, created_by, created_at, updated_at) VALUES (?,?,?,?,?)`,
		g.ID, g.Name, g.CreatedBy, g.CreatedAt, g.UpdatedAt,
	); err != nil {
		return fmt.Errorf("insert group: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO group_members (group_id, user_id, role, share_clock, share_streak, share_adherence, joined_at)
         VALUES (?,?,?,1,1,1,?)`,
		g.ID, g.CreatedBy, "owner", g.CreatedAt,
	); err != nil {
		return fmt.Errorf("insert group owner: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit create group: %w", err)
	}
	return nil
}

// ListGroupsForUser returns every group the user belongs to, with my role and
// member count populated.
func (s *SQLiteStore) ListGroupsForUser(ctx context.Context, userID string) ([]models.Group, error) {
	rows, err := s.db.QueryContext(ctx, `
        SELECT g.id, g.name, g.created_by, g.created_at, g.updated_at, gm.role,
               (SELECT COUNT(*) FROM group_members x WHERE x.group_id = g.id)
        FROM groups g
        JOIN group_members gm ON gm.group_id = g.id
        WHERE gm.user_id = ?
        ORDER BY g.created_at ASC, g.id ASC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list groups for user: %w", err)
	}
	defer rows.Close()
	var out []models.Group
	for rows.Next() {
		var g models.Group
		if err := rows.Scan(&g.ID, &g.Name, &g.CreatedBy, &g.CreatedAt, &g.UpdatedAt, &g.MyRole, &g.MemberCount); err != nil {
			return nil, fmt.Errorf("scan group row: %w", err)
		}
		out = append(out, g)
	}
	return out, nil
}

// GetGroup fetches a group without membership projection.
func (s *SQLiteStore) GetGroup(ctx context.Context, groupID string) (models.Group, error) {
	var g models.Group
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, created_by, created_at, updated_at,
                (SELECT COUNT(*) FROM group_members x WHERE x.group_id = groups.id)
         FROM groups WHERE id = ?`, groupID,
	).Scan(&g.ID, &g.Name, &g.CreatedBy, &g.CreatedAt, &g.UpdatedAt, &g.MemberCount)
	if err == sql.ErrNoRows {
		return models.Group{}, ErrNotFound
	}
	if err != nil {
		return models.Group{}, fmt.Errorf("get group: %w", err)
	}
	return g, nil
}

// IsMember reports whether userID belongs to groupID.
func (s *SQLiteStore) IsMember(ctx context.Context, groupID, userID string) (bool, error) {
	var exists int
	if err := s.db.QueryRowContext(ctx,
		`SELECT CASE WHEN EXISTS (SELECT 1 FROM group_members WHERE group_id=? AND user_id=?) THEN 1 ELSE 0 END`,
		groupID, userID,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("check group membership: %w", err)
	}
	return exists == 1, nil
}

// MemberRole returns the membership role or ErrNotFound.
func (s *SQLiteStore) MemberRole(ctx context.Context, groupID, userID string) (string, error) {
	var role string
	err := s.db.QueryRowContext(ctx,
		`SELECT role FROM group_members WHERE group_id=? AND user_id=?`, groupID, userID,
	).Scan(&role)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get member role: %w", err)
	}
	return role, nil
}

// AddMember inserts a membership, returning ErrAlreadyMember when present.
func (s *SQLiteStore) AddMember(ctx context.Context, groupID, userID, role, joinedAt string) error {
	if role == "" {
		role = "member"
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO group_members (group_id, user_id, role, share_clock, share_streak, share_adherence, joined_at)
         VALUES (?,?,?,1,1,1,?) ON CONFLICT(group_id, user_id) DO NOTHING`,
		groupID, userID, role, joinedAt,
	)
	if err != nil {
		return fmt.Errorf("add group member: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrAlreadyMember
	}
	return nil
}

// RemoveMember deletes a membership and unshares templates that the removed
// member had contributed to the group.
func (s *SQLiteStore) RemoveMember(ctx context.Context, groupID, userID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin remove member: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `DELETE FROM group_members WHERE group_id=? AND user_id=?`, groupID, userID)
	if err != nil {
		return fmt.Errorf("delete group member: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE meal_templates SET group_id='' WHERE group_id=? AND user_id=?`, groupID, userID,
	); err != nil {
		return fmt.Errorf("unshare removed member templates: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit remove member: %w", err)
	}
	return nil
}

// LeaveGroup removes the caller. When the owner leaves, ownership transfers to
// the longest-standing remaining member; if the owner was the last member the
// group, its memberships, invites and template shares are deleted.
func (s *SQLiteStore) LeaveGroup(ctx context.Context, groupID, userID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin leave group: %w", err)
	}
	defer tx.Rollback()

	var role string
	err = tx.QueryRowContext(ctx,
		`SELECT role FROM group_members WHERE group_id=? AND user_id=?`, groupID, userID,
	).Scan(&role)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read leaving member role: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)

	if role == "owner" {
		var nextUser string
		err = tx.QueryRowContext(ctx,
			`SELECT user_id FROM group_members
             WHERE group_id=? AND user_id<>?
             ORDER BY joined_at ASC, user_id ASC LIMIT 1`, groupID, userID,
		).Scan(&nextUser)
		switch {
		case err == sql.ErrNoRows:
			// Last member: unshare all templates and delete the group.
			if _, err := tx.ExecContext(ctx, `UPDATE meal_templates SET group_id='' WHERE group_id=?`, groupID); err != nil {
				return fmt.Errorf("unshare deleted group templates: %w", err)
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM invites WHERE group_id=?`, groupID); err != nil {
				return fmt.Errorf("delete group invites: %w", err)
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM group_members WHERE group_id=?`, groupID); err != nil {
				return fmt.Errorf("delete group members: %w", err)
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM groups WHERE id=?`, groupID); err != nil {
				return fmt.Errorf("delete empty group: %w", err)
			}
			if err := tx.Commit(); err != nil {
				return fmt.Errorf("commit leave last owner: %w", err)
			}
			return nil
		case err != nil:
			return fmt.Errorf("find next owner: %w", err)
		}

		if _, err := tx.ExecContext(ctx,
			`UPDATE group_members SET role='owner' WHERE group_id=? AND user_id=?`, groupID, nextUser,
		); err != nil {
			return fmt.Errorf("promote next owner: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE groups SET created_by=?, updated_at=? WHERE id=?`, nextUser, now, groupID,
		); err != nil {
			return fmt.Errorf("update group owner: %w", err)
		}
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM group_members WHERE group_id=? AND user_id=?`, groupID, userID); err != nil {
		return fmt.Errorf("delete leaving member: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE meal_templates SET group_id='' WHERE group_id=? AND user_id=?`, groupID, userID,
	); err != nil {
		return fmt.Errorf("unshare leaving member templates: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit leave group: %w", err)
	}
	return nil
}

// DeleteGroup removes a group entirely and makes its shared templates private.
func (s *SQLiteStore) DeleteGroup(ctx context.Context, groupID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete group: %w", err)
	}
	defer tx.Rollback()

	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT CASE WHEN EXISTS (SELECT 1 FROM groups WHERE id=?) THEN 1 ELSE 0 END`, groupID).Scan(&exists); err != nil {
		return fmt.Errorf("check group exists: %w", err)
	}
	if exists == 0 {
		return ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `UPDATE meal_templates SET group_id='' WHERE group_id=?`, groupID); err != nil {
		return fmt.Errorf("unshare group templates: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM invites WHERE group_id=?`, groupID); err != nil {
		return fmt.Errorf("delete group invites: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM group_members WHERE group_id=?`, groupID); err != nil {
		return fmt.Errorf("delete group members: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM groups WHERE id=?`, groupID); err != nil {
		return fmt.Errorf("delete group: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete group: %w", err)
	}
	return nil
}

// UpdateMySharing applies a partial update to the caller's sharing toggles.
func (s *SQLiteStore) UpdateMySharing(ctx context.Context, groupID, userID string, shareClock, shareStreak, shareAdherence *bool) error {
	res, err := s.db.ExecContext(ctx, `
        UPDATE group_members SET
          share_clock     = COALESCE(?, share_clock),
          share_streak    = COALESCE(?, share_streak),
          share_adherence = COALESCE(?, share_adherence)
        WHERE group_id=? AND user_id=?`,
		shareClock, shareStreak, shareAdherence, groupID, userID,
	)
	if err != nil {
		return fmt.Errorf("update sharing toggles: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// CreateInvite stores a single-use code.
func (s *SQLiteStore) CreateInvite(ctx context.Context, inv *models.GroupInvite) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO invites (code, group_id, created_by, expires_at, used_by, created_at)
         VALUES (?,?,?,?,?,?)`,
		inv.Code, inv.GroupID, inv.CreatedBy, inv.ExpiresAt, inv.UsedBy, inv.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create invite: %w", err)
	}
	return nil
}

// ConsumeInvite validates a code and adds the caller to the group in one
// transaction. The invite is marked used only after membership is inserted.
func (s *SQLiteStore) ConsumeInvite(ctx context.Context, code, userID, now string) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin consume invite: %w", err)
	}
	defer tx.Rollback()

	var groupID, expiresAt, usedBy string
	err = tx.QueryRowContext(ctx,
		`SELECT group_id, expires_at, COALESCE(used_by,'') FROM invites WHERE code=?`, code,
	).Scan(&groupID, &expiresAt, &usedBy)
	if err == sql.ErrNoRows {
		return "", ErrInviteInvalid
	}
	if err != nil {
		return "", fmt.Errorf("read invite: %w", err)
	}
	if usedBy != "" {
		return "", ErrInviteUsed
	}
	exp, expErr := time.Parse(time.RFC3339, expiresAt)
	nowT, nowErr := time.Parse(time.RFC3339, now)
	if expErr == nil && nowErr == nil && !nowT.Before(exp) {
		return "", ErrInviteExpired
	}

	var member int
	if err := tx.QueryRowContext(ctx,
		`SELECT CASE WHEN EXISTS (SELECT 1 FROM group_members WHERE group_id=? AND user_id=?) THEN 1 ELSE 0 END`,
		groupID, userID,
	).Scan(&member); err != nil {
		return "", fmt.Errorf("check invite membership: %w", err)
	}
	if member == 1 {
		return "", ErrAlreadyMember
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO group_members (group_id, user_id, role, share_clock, share_streak, share_adherence, joined_at)
         VALUES (?,?,'member',1,1,1,?)`, groupID, userID, now,
	); err != nil {
		return "", fmt.Errorf("insert invited member: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE invites SET used_by=? WHERE code=?`, userID, code); err != nil {
		return "", fmt.Errorf("mark invite used: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit consume invite: %w", err)
	}
	return groupID, nil
}

// GroupSummarySource assembles per-member summary data with visibility toggles
// intact. The handler is responsible for projecting it into privacy-safe cards.
func (s *SQLiteStore) GroupSummarySource(ctx context.Context, groupID, today string) ([]GroupMemberSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
        SELECT gm.user_id, COALESCE(NULLIF(p.name,''), 'Member'), gm.role,
               gm.share_clock, gm.share_streak, gm.share_adherence, gm.joined_at
        FROM group_members gm
        LEFT JOIN profiles p ON p.user_id = gm.user_id
        WHERE gm.group_id = ?
        ORDER BY CASE gm.role WHEN 'owner' THEN 0 ELSE 1 END, gm.joined_at ASC, gm.user_id ASC`,
		groupID)
	if err != nil {
		return nil, fmt.Errorf("group summary members: %w", err)
	}
	var out []GroupMemberSummary
	for rows.Next() {
		var m GroupMemberSummary
		var clock, streak, adherence int
		if err := rows.Scan(&m.UserID, &m.Name, &m.Role, &clock, &streak, &adherence, &m.JoinedAt); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan group summary member: %w", err)
		}
		m.ShareClock = clock == 1
		m.ShareStreak = streak == 1
		m.ShareAdherence = adherence == 1
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate group summary members: %w", err)
	}
	rows.Close()

	todayTime, err := time.Parse("2006-01-02", today)
	if err != nil {
		todayTime = time.Now().UTC()
		today = todayTime.Format("2006-01-02")
	}
	since := todayTime.AddDate(0, 0, -29).Format("2006-01-02")

	for i := range out {
		m := &out[i]
		logs, err := s.FetchNutritionLogsRange(ctx, m.UserID, since, today)
		if err != nil {
			return nil, fmt.Errorf("group summary nutrition for %s: %w", m.UserID, err)
		}
		loggedDays, streak, _ := metrics.LoggingStreaks(logs, today, 30)
		m.Streak = streak
		loggedCount := 0
		for _, logged := range loggedDays {
			if logged {
				loggedCount++
			}
		}
		m.AdherencePct = math.Round(float64(loggedCount)/float64(len(loggedDays))*1000) / 10
		for _, l := range logs {
			if l.Date == today && (l.Calories > 0 || l.ProteinG > 0 || l.CarbsG > 0 || l.FatG > 0 || l.FiberG > 0) {
				m.LoggedFood = true
				break
			}
		}

		var weightCount int
		if err := s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM biometric_logs WHERE user_id=? AND date=? AND weight_kg > 0`,
			m.UserID, today,
		).Scan(&weightCount); err != nil {
			return nil, fmt.Errorf("group summary weight for %s: %w", m.UserID, err)
		}
		m.LoggedWeight = weightCount > 0

		var workoutCount int
		if err := s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM workouts WHERE user_id=? AND date=?`, m.UserID, today,
		).Scan(&workoutCount); err != nil {
			return nil, fmt.Errorf("group summary workout for %s: %w", m.UserID, err)
		}
		m.LoggedWorkout = workoutCount > 0
	}
	return out, nil
}

// ShareTemplateToGroup shares a caller-owned template with a group the caller
// belongs to.
func (s *SQLiteStore) ShareTemplateToGroup(ctx context.Context, userID, templateID, groupID string) error {
	if groupID == "" {
		return ErrNotFound
	}
	var ownerID string
	err := s.db.QueryRowContext(ctx,
		`SELECT user_id FROM meal_templates WHERE id=?`, templateID,
	).Scan(&ownerID)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read template owner: %w", err)
	}
	if ownerID != userID {
		return ErrForbidden
	}
	member, err := s.IsMember(ctx, groupID, userID)
	if err != nil {
		return err
	}
	if !member {
		return ErrForbidden
	}
	if _, err := s.db.ExecContext(ctx,
		`UPDATE meal_templates SET group_id=?, updated_at=? WHERE id=? AND user_id=?`,
		groupID, time.Now().UTC().Format(time.RFC3339), templateID, userID,
	); err != nil {
		return fmt.Errorf("share template: %w", err)
	}
	return nil
}

// UnshareTemplate makes a template private. Allowed for the template owner or
// the owner of the group it is shared with.
func (s *SQLiteStore) UnshareTemplate(ctx context.Context, actorID, templateID string) error {
	var ownerID, groupID string
	err := s.db.QueryRowContext(ctx,
		`SELECT user_id, group_id FROM meal_templates WHERE id=?`, templateID,
	).Scan(&ownerID, &groupID)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("read template share: %w", err)
	}
	if groupID == "" {
		return nil
	}
	if actorID != ownerID {
		role, err := s.MemberRole(ctx, groupID, actorID)
		if err != nil || role != "owner" {
			if err == ErrNotFound {
				return ErrForbidden
			}
			if err != nil {
				return err
			}
			return ErrForbidden
		}
	}
	if _, err := s.db.ExecContext(ctx,
		`UPDATE meal_templates SET group_id='', updated_at=? WHERE id=?`,
		time.Now().UTC().Format(time.RFC3339), templateID,
	); err != nil {
		return fmt.Errorf("unshare template: %w", err)
	}
	return nil
}
