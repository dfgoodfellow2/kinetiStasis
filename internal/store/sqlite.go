package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strings"
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
	// weight_before/weight_after/calories_before/calories_after/reason were
	// added as nullable columns by 004; legacy rows can carry SQL NULL, so
	// every reader must COALESCE them before scanning into value types.
	err := s.db.QueryRowContext(ctx, `SELECT id, user_id, check_in_date, COALESCE(weight_before,0), COALESCE(weight_after,0), COALESCE(calories_before,0), COALESCE(calories_after,0), COALESCE(reason,''), created_at, observed_tdee, tdee_confidence, lookback_days FROM check_in_logs WHERE user_id = ? ORDER BY check_in_date DESC LIMIT 1`, userID).Scan(&out.ID, &out.UserID, &out.CheckInDate, &out.WeightBefore, &out.WeightAfter, &out.CaloriesBefore, &out.CaloriesAfter, &out.Reason, &out.CreatedAt, &out.ObservedTDEE, &out.TDEEConfidence, &out.LookbackDays)
	if err != nil {
		return out, fmt.Errorf("get last checkin: %w", err)
	}
	return out, nil
}

// execer is the write subset shared by *sql.DB and *sql.Tx. Extracting it lets
// the legacy single-write methods and RecordCheckinTx share exactly the same
// SQL without duplicating statements.
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// insertCheckinLog writes the check-in row using the supplied executor.
func insertCheckinLog(ctx context.Context, ex execer, c *models.CheckInLog) error {
	_, err := ex.ExecContext(ctx, `INSERT INTO check_in_logs (id, user_id, check_in_date, weight_before, weight_after, calories_before, calories_after, reason, created_at, observed_tdee, tdee_confidence, lookback_days) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`, c.ID, c.UserID, c.CheckInDate, c.WeightBefore, c.WeightAfter, c.CaloriesBefore, c.CaloriesAfter, c.Reason, c.CreatedAt, c.ObservedTDEE, c.TDEEConfidence, c.LookbackDays)
	return err
}

// CreateCheckinLog inserts a new check-in row.
func (s *SQLiteStore) CreateCheckinLog(ctx context.Context, c *models.CheckInLog) error {
	if err := insertCheckinLog(ctx, s.db, c); err != nil {
		return fmt.Errorf("create checkin log: %w", err)
	}
	return nil
}

// RecordCheckinTx persists the check-in row, an optional target update and an
// optional pre-change target snapshot in a single transaction. A nil targets
// leaves the targets table untouched (the policy-blocked path); a nil snap
// skips the history row. The check-in insert error is returned unwrapped so
// callers can match the driver's exact UNIQUE-constraint text for the
// duplicate-day check-in conflict.
func (s *SQLiteStore) RecordCheckinTx(ctx context.Context, c *models.CheckInLog, targets *models.Targets, snap *models.TargetSnapshot) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin record checkin: %w", err)
	}
	defer tx.Rollback()

	if err := insertCheckinLog(ctx, tx, c); err != nil {
		return err
	}
	if targets != nil {
		if err := upsertTargets(ctx, tx, targets); err != nil {
			return fmt.Errorf("upsert targets: %w", err)
		}
	}
	if snap != nil {
		if err := insertTargetSnapshot(ctx, tx, snap); err != nil {
			return fmt.Errorf("create target snapshot: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit record checkin: %w", err)
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

// SumWorkoutCaloriesForDate returns the total calories burned by all workouts
// logged on a single date. No rows yields 0, not an error.
func (s *SQLiteStore) SumWorkoutCaloriesForDate(ctx context.Context, userID, date string) (float64, error) {
	var total float64
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(calories_burned),0) FROM workouts WHERE user_id=? AND date=?`, userID, date).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("sum workout calories for date: %w", err)
	}
	return total, nil
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

func (s *SQLiteStore) CreateUser(ctx context.Context, id, username, email, passwordHash string, isAdmin, pendingApproval bool, createdAt, updatedAt string) error {
	admin := 0
	if isAdmin {
		admin = 1
	}
	pending := 0
	if pendingApproval {
		pending = 1
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO users (id, username, email, password, is_admin, pending_approval, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?)`, id, username, email, passwordHash, admin, pending, createdAt, updatedAt)
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
	var isAdmin, pending int
	if err := s.db.QueryRowContext(ctx, `SELECT id, username, email, is_admin, pending_approval, created_at, updated_at FROM users WHERE id = ?`, userID).Scan(&u.ID, &u.Username, &u.Email, &isAdmin, &pending, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return u, fmt.Errorf("get user by id: %w", err)
	}
	u.IsAdmin = isAdmin == 1
	u.PendingApproval = pending == 1
	return u, nil
}

// GetUserPasswordHash returns the bcrypt hash stored for a user. Callers must
// never log the returned value.
func (s *SQLiteStore) GetUserPasswordHash(ctx context.Context, userID string) (string, error) {
	var hash string
	if err := s.db.QueryRowContext(ctx, `SELECT password FROM users WHERE id = ?`, userID).Scan(&hash); err != nil {
		return "", fmt.Errorf("get user password hash: %w", err)
	}
	return hash, nil
}

// UpdateUserPassword replaces the stored bcrypt hash and bumps updated_at.
func (s *SQLiteStore) UpdateUserPassword(ctx context.Context, userID, passwordHash string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE users SET password = ?, updated_at = ? WHERE id = ?`,
		passwordHash, time.Now().UTC().Format(time.RFC3339), userID)
	if err != nil {
		return fmt.Errorf("update user password: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update user password rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
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

// DeleteUserRefreshTokens revokes every refresh-token row for a user. Used on
// password change/reset so sessions on other devices cannot be refreshed.
func (s *SQLiteStore) DeleteUserRefreshTokens(ctx context.Context, userID string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("delete user refresh tokens: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ListUsers(ctx context.Context) ([]models.User, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, username, email, is_admin, pending_approval, created_at, updated_at FROM users ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()
	var out []models.User
	for rows.Next() {
		var u models.User
		var isAdmin, pending int
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &isAdmin, &pending, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan user row: %w", err)
		}
		u.IsAdmin = isAdmin == 1
		u.PendingApproval = pending == 1
		out = append(out, u)
	}
	return out, nil
}

func (s *SQLiteStore) DeleteUser(ctx context.Context, userID string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, userID)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete user rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
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

// --- Registration gate -------------------------------------------------------

// AddSetting upserts an instance_settings row. The only key used today is
// registration_mode, but the table is intentionally generic.
func (s *SQLiteStore) AddSetting(ctx context.Context, key, value string) error {
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO instance_settings ("key", value) VALUES (?,?)
         ON CONFLICT("key") DO UPDATE SET value = excluded.value`,
		key, value,
	); err != nil {
		return fmt.Errorf("add setting %q: %w", key, err)
	}
	return nil
}

// GetSetting returns the stored value. sql.ErrNoRows is preserved via %w so
// callers can distinguish a missing setting from a real database failure.
func (s *SQLiteStore) GetSetting(ctx context.Context, key string) (string, error) {
	var value string
	if err := s.db.QueryRowContext(ctx,
		`SELECT value FROM instance_settings WHERE "key" = ?`, key,
	).Scan(&value); err != nil {
		return "", fmt.Errorf("get setting %q: %w", key, err)
	}
	return value, nil
}

func (s *SQLiteStore) CreateRegistrationInvite(ctx context.Context, inv *models.RegistrationInvite) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO registration_invites (code, created_by, expires_at, used_by, created_at)
         VALUES (?,?,?,?,?)`,
		inv.Code, inv.CreatedBy, inv.ExpiresAt, inv.UsedBy, inv.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create registration invite: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ListRegistrationInvites(ctx context.Context) ([]models.RegistrationInvite, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT code, created_by, expires_at, COALESCE(used_by,''), created_at
         FROM registration_invites ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list registration invites: %w", err)
	}
	defer rows.Close()

	var out []models.RegistrationInvite
	for rows.Next() {
		var inv models.RegistrationInvite
		if err := rows.Scan(&inv.Code, &inv.CreatedBy, &inv.ExpiresAt, &inv.UsedBy, &inv.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan registration invite: %w", err)
		}
		out = append(out, inv)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate registration invites: %w", err)
	}
	return out, nil
}

func (s *SQLiteStore) RevokeRegistrationInvite(ctx context.Context, code string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM registration_invites WHERE code = ?`, code)
	if err != nil {
		return fmt.Errorf("revoke registration invite: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("revoke registration invite rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ConsumeRegistrationInvite validates a code and marks it used in a single
// transaction, mirroring ConsumeInvite for groups. The caller supplies the
// account ID that will own the code so used_by is populated atomically.
func (s *SQLiteStore) ConsumeRegistrationInvite(ctx context.Context, code, userID, now string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin consume registration invite: %w", err)
	}
	defer tx.Rollback()

	var expiresAt, usedBy string
	err = tx.QueryRowContext(ctx,
		`SELECT expires_at, COALESCE(used_by,'') FROM registration_invites WHERE code = ?`, code,
	).Scan(&expiresAt, &usedBy)
	if err == sql.ErrNoRows {
		return ErrInviteInvalid
	}
	if err != nil {
		return fmt.Errorf("read registration invite: %w", err)
	}
	if usedBy != "" {
		return ErrInviteUsed
	}
	// Fail closed: an unparseable stored expiry (or clock value) must never let
	// a code through. Legacy/malformed rows are treated as expired.
	exp, expErr := time.Parse(time.RFC3339, expiresAt)
	nowT, nowErr := time.Parse(time.RFC3339, now)
	if expErr != nil || nowErr != nil || !nowT.Before(exp) {
		return ErrInviteExpired
	}

	res, err := tx.ExecContext(ctx,
		`UPDATE registration_invites SET used_by = ? WHERE code = ? AND COALESCE(used_by,'') = ''`,
		userID, code,
	)
	if err != nil {
		return fmt.Errorf("mark registration invite used: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mark registration invite rows affected: %w", err)
	}
	if n == 0 {
		// Lost a race with another registration.
		return ErrInviteUsed
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit consume registration invite: %w", err)
	}
	return nil
}

// CompensateRegistrationInvite un-burns a code when the account creation that
// followed its consumption failed. It only clears used_by when the recorded
// owner is this exact attempt, so it can never release a code won by another
// concurrent registration.
func (s *SQLiteStore) CompensateRegistrationInvite(ctx context.Context, code, userID string) error {
	if _, err := s.db.ExecContext(ctx,
		`UPDATE registration_invites SET used_by = '' WHERE code = ? AND used_by = ?`,
		code, userID,
	); err != nil {
		return fmt.Errorf("compensate registration invite: %w", err)
	}
	return nil
}

// ListPendingUsers returns accounts created under public_approval that have
// not yet been approved. Rows are ordered oldest-first so admins review the
// queue in arrival order.
func (s *SQLiteStore) ListPendingUsers(ctx context.Context) ([]models.User, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, username, email, is_admin, pending_approval, created_at, updated_at
         FROM users WHERE pending_approval = 1 ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list pending users: %w", err)
	}
	defer rows.Close()

	var out []models.User
	for rows.Next() {
		var u models.User
		var isAdmin, pending int
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &isAdmin, &pending, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan pending user: %w", err)
		}
		u.IsAdmin = isAdmin == 1
		u.PendingApproval = pending == 1
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending users: %w", err)
	}
	return out, nil
}

func (s *SQLiteStore) IsUserPendingApproval(ctx context.Context, userID string) (bool, error) {
	var pending int
	if err := s.db.QueryRowContext(ctx,
		`SELECT pending_approval FROM users WHERE id = ?`, userID,
	).Scan(&pending); err != nil {
		return false, fmt.Errorf("get pending approval: %w", err)
	}
	return pending == 1, nil
}

func (s *SQLiteStore) SetUserPendingApproval(ctx context.Context, userID string, pending bool) error {
	flag := 0
	if pending {
		flag = 1
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET pending_approval = ?, updated_at = ? WHERE id = ?`,
		flag, time.Now().UTC().Format(time.RFC3339), userID,
	)
	if err != nil {
		return fmt.Errorf("set pending approval: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("set pending approval rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// --- Nutrition ---
func (s *SQLiteStore) FetchNutritionLogsRange(ctx context.Context, userID, from, to string) ([]models.NutritionLog, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,user_id,date,calories,protein_g,carbs_g,fat_g,fiber_g,water_ml,COALESCE(meal_notes,''),updated_at FROM nutrition_logs WHERE user_id = ? AND date >= ? AND date <= ? ORDER BY date ASC`, userID, from, to)
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
	err := s.db.QueryRowContext(ctx, `SELECT id,user_id,date,calories,protein_g,carbs_g,fat_g,fiber_g,water_ml,COALESCE(meal_notes,''),updated_at FROM nutrition_logs WHERE user_id = ? AND date = ?`, userID, date).Scan(&n.ID, &n.UserID, &n.Date, &n.Calories, &n.ProteinG, &n.CarbsG, &n.FatG, &n.FiberG, &n.WaterMl, &n.MealNotes, &n.UpdatedAt)
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
	rows, err := s.db.QueryContext(ctx, `SELECT id,user_id,date,COALESCE(weight_kg,0),COALESCE(waist_cm,0),COALESCE(grip_kg,0),COALESCE(bolt_score,0),COALESCE(sleep_hours,0),COALESCE(sleep_quality,0),COALESCE(subjective_feel,0),COALESCE(body_fat_pct,0),COALESCE(notes,''),updated_at FROM biometric_logs WHERE user_id = ? AND date >= ? AND date <= ? ORDER BY date ASC`, userID, from, to)
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
	err := s.db.QueryRowContext(ctx, `SELECT id,user_id,date,COALESCE(weight_kg,0),COALESCE(waist_cm,0),COALESCE(grip_kg,0),COALESCE(bolt_score,0),COALESCE(sleep_hours,0),COALESCE(sleep_quality,0),COALESCE(subjective_feel,0),COALESCE(body_fat_pct,0),COALESCE(notes,''),updated_at FROM biometric_logs WHERE user_id = ? AND date = ?`, userID, date).Scan(&b.ID, &b.UserID, &b.Date, &b.WeightKg, &b.WaistCm, &b.GripKg, &b.BoltScore, &b.SleepHours, &b.SleepQuality, &b.SubjectiveFeel, &b.BodyFatPct, &b.Notes, &b.UpdatedAt)
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
				return nil, fmt.Errorf("%w: exercises_json for %s/%s: %v", ErrCorruptWorkoutData, w.Date, w.Slot, err)
			}
			for i := range w.Exercises {
				if w.Exercises[i].DurationRaw == "" && w.Exercises[i].DurationRawLegacy != "" {
					w.Exercises[i].DurationRaw = w.Exercises[i].DurationRawLegacy
				}
			}
		}
		if metadataJSON != "" && metadataJSON != "{}" {
			if err := json.Unmarshal([]byte(metadataJSON), &w.Metadata); err != nil {
				return nil, fmt.Errorf("%w: metadata_json for %s/%s: %v", ErrCorruptWorkoutData, w.Date, w.Slot, err)
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
			return w, fmt.Errorf("%w: exercises_json for %s/%s: %v", ErrCorruptWorkoutData, date, slot, err)
		}
		for i := range w.Exercises {
			if w.Exercises[i].DurationRaw == "" && w.Exercises[i].DurationRawLegacy != "" {
				w.Exercises[i].DurationRaw = w.Exercises[i].DurationRawLegacy
			}
		}
	}
	if metadataJSON != "" {
		if err := json.Unmarshal([]byte(metadataJSON), &w.Metadata); err != nil {
			return w, fmt.Errorf("%w: metadata_json for %s/%s: %v", ErrCorruptWorkoutData, date, slot, err)
		}
	}
	return w, nil
}

func (s *SQLiteStore) UpsertWorkout(ctx context.Context, w *models.WorkoutEntry) error {
	exb, err := json.Marshal(w.Exercises)
	if err != nil {
		return fmt.Errorf("upsert workout marshal exercises: %w", err)
	}
	metb, err := json.Marshal(w.Metadata)
	if err != nil {
		return fmt.Errorf("upsert workout marshal metadata: %w", err)
	}
	// A single statement keeps the update-then-insert path atomic: two
	// concurrent writers for the same (user_id,date,slot) can no longer race
	// between an empty UPDATE and an INSERT that trips the UNIQUE constraint.
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO workouts (id,user_id,date,slot,title,raw_notes,duration_min,calories_burned,mwv,nds,session_density,exercises_json,metadata_json,updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(user_id,date,slot) DO UPDATE SET
			title           = excluded.title,
			raw_notes       = excluded.raw_notes,
			duration_min    = excluded.duration_min,
			calories_burned = excluded.calories_burned,
			mwv             = excluded.mwv,
			nds             = excluded.nds,
			session_density = excluded.session_density,
			exercises_json  = excluded.exercises_json,
			metadata_json   = excluded.metadata_json,
			updated_at      = excluded.updated_at`,
		w.ID, w.UserID, w.Date, w.Slot, w.Title, w.RawNotes, w.DurationMin, w.CaloriesBurned,
		w.MWV, w.NDS, w.SessionDensity, string(exb), string(metb), w.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert workout: %w", err)
	}
	return nil
}

func (s *SQLiteStore) UpdateWorkout(ctx context.Context, w *models.WorkoutEntry) (models.WorkoutEntry, error) {
	exb, err := json.Marshal(w.Exercises)
	if err != nil {
		return models.WorkoutEntry{}, fmt.Errorf("update workout marshal exercises: %w", err)
	}
	metb, err := json.Marshal(w.Metadata)
	if err != nil {
		return models.WorkoutEntry{}, fmt.Errorf("update workout marshal metadata: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `UPDATE workouts SET title=?,raw_notes=?,duration_min=?,calories_burned=?,mwv=?,nds=?,session_density=?,exercises_json=?,metadata_json=?,updated_at=? WHERE user_id=? AND date=? AND slot=?`, w.Title, w.RawNotes, w.DurationMin, w.CaloriesBurned, w.MWV, w.NDS, w.SessionDensity, string(exb), string(metb), w.UpdatedAt, w.UserID, w.Date, w.Slot)
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

// upsertTargets writes the full target row using the supplied executor.
func upsertTargets(ctx context.Context, ex execer, t *models.Targets) error {
	eatBack := 0
	if t.EatBackExercise {
		eatBack = 1
	}
	_, err := ex.ExecContext(ctx, `INSERT INTO targets (user_id,calories,protein_g,carbs_g,fat_g,fiber_g,water_ml,eat_back_exercise,updated_at) VALUES (?,?,?,?,?,?,?,?,?) ON CONFLICT(user_id) DO UPDATE SET calories=excluded.calories,protein_g=excluded.protein_g,carbs_g=excluded.carbs_g,fat_g=excluded.fat_g,fiber_g=excluded.fiber_g,water_ml=excluded.water_ml,eat_back_exercise=excluded.eat_back_exercise,updated_at=excluded.updated_at`, t.UserID, t.Calories, t.ProteinG, t.CarbsG, t.FatG, t.FiberG, t.WaterMl, eatBack, t.UpdatedAt)
	return err
}

func (s *SQLiteStore) UpsertTargets(ctx context.Context, t *models.Targets) error {
	if err := upsertTargets(ctx, s.db, t); err != nil {
		return fmt.Errorf("upsert targets: %w", err)
	}
	return nil
}

// RecordTargetUpdateTx writes the live target row and an optional pre-change
// snapshot atomically. The manual-edit path previously called UpsertTargets and
// CreateTargetSnapshot separately, so a failure between them could publish a
// new target with no history row (or vice versa).
func (s *SQLiteStore) RecordTargetUpdateTx(ctx context.Context, t *models.Targets, snap *models.TargetSnapshot) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin record target update: %w", err)
	}
	defer tx.Rollback()

	if err := upsertTargets(ctx, tx, t); err != nil {
		return fmt.Errorf("upsert targets: %w", err)
	}
	if snap != nil {
		if err := insertTargetSnapshot(ctx, tx, snap); err != nil {
			return fmt.Errorf("create target snapshot: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit record target update: %w", err)
	}
	return nil
}

// insertTargetSnapshot writes a pre-change target row using the supplied
// executor.
func insertTargetSnapshot(ctx context.Context, ex execer, snap *models.TargetSnapshot) error {
	eatBack := 0
	if snap.EatBackExercise {
		eatBack = 1
	}
	_, err := ex.ExecContext(ctx, `INSERT INTO target_history (id,user_id,effective_date,calories,protein_g,carbs_g,fat_g,fiber_g,water_ml,eat_back_exercise,created_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)`, snap.ID, snap.UserID, snap.EffectiveDate, snap.Calories, snap.ProteinG, snap.CarbsG, snap.FatG, snap.FiberG, snap.WaterMl, eatBack, snap.CreatedAt)
	return err
}

func (s *SQLiteStore) CreateTargetSnapshot(ctx context.Context, snap *models.TargetSnapshot) error {
	if err := insertTargetSnapshot(ctx, s.db, snap); err != nil {
		return fmt.Errorf("create target snapshot: %w", err)
	}
	return nil
}

func (s *SQLiteStore) FetchTargetHistory(ctx context.Context, userID string) ([]models.TargetSnapshot, error) {
	// created_at is the tie-breaker: several snapshots can share an
	// effective_date (check-in + manual edit), and without a secondary key
	// their relative order is unspecified.
	rows, err := s.db.QueryContext(ctx, `SELECT id,user_id,effective_date,calories,protein_g,carbs_g,fat_g,fiber_g,water_ml,eat_back_exercise,created_at FROM target_history WHERE user_id = ? ORDER BY effective_date DESC, created_at DESC`, userID)
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
	rows, err := s.db.QueryContext(ctx, `SELECT id,user_id,date,
        COALESCE(neck_cm,0), COALESCE(chest_cm,0), COALESCE(waist_cm,0),
        COALESCE(hips_cm,0), COALESCE(thigh_cm,0), COALESCE(bicep_cm,0),
        COALESCE(notes,''), created_at,
        COALESCE(shoulders_cm,0), COALESCE(calves_cm,0)
        FROM body_measurements WHERE user_id = ? AND date >= ? AND date <= ? ORDER BY date ASC`, userID, from, to)
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
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate measurement rows: %w", err)
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
	res, err := s.db.ExecContext(ctx, `UPDATE body_measurements SET neck_cm=?,chest_cm=?,waist_cm=?,hips_cm=?,thigh_cm=?,bicep_cm=?,notes=?,shoulders_cm=?,calves_cm=? WHERE user_id=? AND date=?`, m.NeckCm, m.ChestCm, m.WaistCm, m.HipsCm, m.ThighCm, m.BicepCm, m.Notes, m.ShouldersCm, m.CalvesCm, m.UserID, m.Date)
	if err != nil {
		return fmt.Errorf("update measurement: %w", err)
	}
	// PUT is a replace of an existing row: zero affected rows means the
	// (user, date) pair does not exist, which the handler maps to 404 rather
	// than reporting a phantom success.
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update measurement rows affected: %w", err)
	}
	if n == 0 {
		return ErrNotFound
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
	// Fail closed: a malformed expires_at is invalid, never an open door.
	exp, expErr := time.Parse(time.RFC3339, expiresAt)
	nowT, nowErr := time.Parse(time.RFC3339, now)
	if expErr != nil || nowErr != nil || !nowT.Before(exp) {
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

// hasNutritionContent reports whether a nutrition row represents actual food
// intake. Water-only rows (waterMl set while every macro and calorie is zero)
// are hydration logs, not logged food days, so they must not count toward
// adherence or streaks — the same predicate used for the LoggedFood presence
// flag on member cards.
func hasNutritionContent(l models.NutritionLog) bool {
	return l.Calories > 0 || l.ProteinG > 0 || l.CarbsG > 0 || l.FatG > 0 || l.FiberG > 0
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
		// Adherence counts days with actual food intake, mirroring the
		// LoggedFood predicate: water-only rows are not logged days.
		foodLogs := make([]models.NutritionLog, 0, len(logs))
		for _, l := range logs {
			if hasNutritionContent(l) {
				foodLogs = append(foodLogs, l)
			}
		}
		loggedDays, streak, _ := metrics.LoggingStreaks(foodLogs, today, 30)
		m.Streak = streak
		loggedCount := 0
		for _, logged := range loggedDays {
			if logged {
				loggedCount++
			}
		}
		m.AdherencePct = math.Round(float64(loggedCount)/float64(len(loggedDays))*1000) / 10
		for _, l := range foodLogs {
			if l.Date == today {
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

// --- Admin data-integrity report -------------------------------------------

// integrityUsernameExpr resolves the household-facing identity for a row.
// Profile names are preferred because that is what the admin sees elsewhere;
// the login username is the fallback for accounts that never filled a profile.
// Emails and password hashes are never selected.
const integrityUsernameExpr = `COALESCE(NULLIF(TRIM(p.name),''), NULLIF(TRIM(u.username),''), '')`

// IntegrityReport runs every anomaly check in a single read-only pass. It is
// intentionally resilient: a query failure is appended to errors and the
// remaining checks still run, so a partially migrated production DB always
// produces a usable report.
func (s *SQLiteStore) IntegrityReport(ctx context.Context) ([]IntegrityFinding, []string) {
	findings := make([]IntegrityFinding, 0, 10)
	errs := make([]string, 0)

	add := func(check, countQuery, sampleQuery string) {
		finding, err := s.integrityFinding(ctx, check, countQuery, sampleQuery)
		if err != nil {
			errs = append(errs, check+": "+err.Error())
			return
		}
		findings = append(findings, finding)
	}

	// 1. Doubled or unparseable nutrition days: a stored calorie total that
	// disagrees with the macro-implied energy by more than 25%.
	add("nutrition_macro_mismatch", `
        SELECT COUNT(*) FROM nutrition_logs n
        WHERE n.calories > 300
          AND ABS(n.calories - (COALESCE(n.protein_g,0)*4 + COALESCE(n.carbs_g,0)*4 + COALESCE(n.fat_g,0)*9)) > 0.25 * n.calories`,
		`
        SELECT `+integrityUsernameExpr+` AS username,
               n.date AS date,
               n.calories AS calories,
               (COALESCE(n.protein_g,0)*4 + COALESCE(n.carbs_g,0)*4 + COALESCE(n.fat_g,0)*9) AS macroKcal
        FROM nutrition_logs n
        LEFT JOIN users u ON u.id = n.user_id
        LEFT JOIN profiles p ON p.user_id = u.id
        WHERE n.calories > 300
          AND ABS(n.calories - (COALESCE(n.protein_g,0)*4 + COALESCE(n.carbs_g,0)*4 + COALESCE(n.fat_g,0)*9)) > 0.25 * n.calories
        ORDER BY n.date DESC
        LIMIT 5`)

	// 2. Meal notes that repeat the same line or long sentence fragment.
	// SQLite has no regex/greatest-n helper, so this one check fetches the
	// candidate rows and applies the duplicate-line test in Go.
	if finding, err := s.integrityDoubledNotes(ctx); err != nil {
		errs = append(errs, "nutrition_doubled_notes: "+err.Error())
	} else {
		findings = append(findings, finding)
	}

	// 3. Non-empty days outside a human calorie/water envelope.
	add("nutrition_implausible", `
        SELECT COUNT(*) FROM nutrition_logs n
        WHERE n.calories > 6000
           OR (n.calories > 0 AND n.calories < 500)
           OR COALESCE(n.water_ml,0) > 8000`,
		`
        SELECT `+integrityUsernameExpr+` AS username,
               n.date AS date,
               n.calories AS calories,
               n.water_ml AS waterMl
        FROM nutrition_logs n
        LEFT JOIN users u ON u.id = n.user_id
        LEFT JOIN profiles p ON p.user_id = u.id
        WHERE n.calories > 6000
           OR (n.calories > 0 AND n.calories < 500)
           OR COALESCE(n.water_ml,0) > 8000
        ORDER BY n.date DESC
        LIMIT 5`)

	// 4. Raise-era check-in signatures: a raise above any plausible target, or
	// an unchanged calorie value recorded before the fix landed.
	add("checkin_raise_era", `
        SELECT COUNT(*) FROM check_in_logs c
        WHERE COALESCE(c.calories_after,0) > 4500
           OR (COALESCE(c.calories_after,0) = COALESCE(c.calories_before,0)
               AND c.created_at < '2026-11-01')`,
		`
        SELECT `+integrityUsernameExpr+` AS username,
               c.check_in_date AS date,
               c.calories_before AS caloriesBefore,
               c.calories_after AS caloriesAfter,
               c.created_at AS createdAt
        FROM check_in_logs c
        LEFT JOIN users u ON u.id = c.user_id
        LEFT JOIN profiles p ON p.user_id = u.id
        WHERE COALESCE(c.calories_after,0) > 4500
           OR (COALESCE(c.calories_after,0) = COALESCE(c.calories_before,0)
               AND c.created_at < '2026-11-01')
        ORDER BY c.created_at DESC
        LIMIT 5`)

	// 5. Target-history snapshots outside a sane calorie range.
	add("target_history_implausible", `
        SELECT COUNT(*) FROM target_history t
        WHERE t.calories > 6000 OR t.calories < 1000`,
		`
        SELECT `+integrityUsernameExpr+` AS username,
               t.effective_date AS effectiveDate,
               t.calories AS calories
        FROM target_history t
        LEFT JOIN users u ON u.id = t.user_id
        LEFT JOIN profiles p ON p.user_id = u.id
        WHERE t.calories > 6000 OR t.calories < 1000
        ORDER BY t.effective_date DESC
        LIMIT 5`)

	// 6. Unit-corrupted circumferences. Zero means "not logged" (migrations
	// 013/001 backfill sentinel), so only positive values below 5 cm count.
	add("measurement_implausible", `
        SELECT COUNT(*) FROM body_measurements m
        WHERE (COALESCE(m.neck_cm,0) > 250      OR (COALESCE(m.neck_cm,0)      > 0 AND COALESCE(m.neck_cm,0)      < 5))
           OR (COALESCE(m.chest_cm,0) > 250     OR (COALESCE(m.chest_cm,0)     > 0 AND COALESCE(m.chest_cm,0)     < 5))
           OR (COALESCE(m.waist_cm,0) > 250     OR (COALESCE(m.waist_cm,0)     > 0 AND COALESCE(m.waist_cm,0)     < 5))
           OR (COALESCE(m.hips_cm,0) > 250      OR (COALESCE(m.hips_cm,0)      > 0 AND COALESCE(m.hips_cm,0)      < 5))
           OR (COALESCE(m.thigh_cm,0) > 250     OR (COALESCE(m.thigh_cm,0)     > 0 AND COALESCE(m.thigh_cm,0)     < 5))
           OR (COALESCE(m.bicep_cm,0) > 250     OR (COALESCE(m.bicep_cm,0)     > 0 AND COALESCE(m.bicep_cm,0)     < 5))
           OR (COALESCE(m.shoulders_cm,0) > 250 OR (COALESCE(m.shoulders_cm,0) > 0 AND COALESCE(m.shoulders_cm,0) < 5))
           OR (COALESCE(m.calves_cm,0) > 250    OR (COALESCE(m.calves_cm,0)    > 0 AND COALESCE(m.calves_cm,0)    < 5))`,
		`
        SELECT `+integrityUsernameExpr+` AS username,
               m.date AS date,
               m.neck_cm AS neckCm,
               m.chest_cm AS chestCm,
               m.waist_cm AS waistCm,
               m.hips_cm AS hipsCm,
               m.thigh_cm AS thighCm,
               m.bicep_cm AS bicepCm,
               m.shoulders_cm AS shouldersCm,
               m.calves_cm AS calvesCm
        FROM body_measurements m
        LEFT JOIN users u ON u.id = m.user_id
        LEFT JOIN profiles p ON p.user_id = u.id
        WHERE (COALESCE(m.neck_cm,0) > 250      OR (COALESCE(m.neck_cm,0)      > 0 AND COALESCE(m.neck_cm,0)      < 5))
           OR (COALESCE(m.chest_cm,0) > 250     OR (COALESCE(m.chest_cm,0)     > 0 AND COALESCE(m.chest_cm,0)     < 5))
           OR (COALESCE(m.waist_cm,0) > 250     OR (COALESCE(m.waist_cm,0)     > 0 AND COALESCE(m.waist_cm,0)     < 5))
           OR (COALESCE(m.hips_cm,0) > 250      OR (COALESCE(m.hips_cm,0)      > 0 AND COALESCE(m.hips_cm,0)      < 5))
           OR (COALESCE(m.thigh_cm,0) > 250     OR (COALESCE(m.thigh_cm,0)     > 0 AND COALESCE(m.thigh_cm,0)     < 5))
           OR (COALESCE(m.bicep_cm,0) > 250     OR (COALESCE(m.bicep_cm,0)     > 0 AND COALESCE(m.bicep_cm,0)     < 5))
           OR (COALESCE(m.shoulders_cm,0) > 250 OR (COALESCE(m.shoulders_cm,0) > 0 AND COALESCE(m.shoulders_cm,0) < 5))
           OR (COALESCE(m.calves_cm,0) > 250    OR (COALESCE(m.calves_cm,0)    > 0 AND COALESCE(m.calves_cm,0)    < 5))
        ORDER BY m.date DESC
        LIMIT 5`)

	// 7. Biometric values outside human boundaries. Weight 0 means "not logged".
	add("biometric_implausible", `
        SELECT COUNT(*) FROM biometric_logs b
        WHERE (COALESCE(b.weight_kg,0) > 0 AND (COALESCE(b.weight_kg,0) < 20 OR COALESCE(b.weight_kg,0) > 300))
           OR COALESCE(b.grip_kg,0) > 100
           OR COALESCE(b.bolt_score,0) > 120
           OR COALESCE(b.sleep_hours,0) > 16`,
		`
        SELECT `+integrityUsernameExpr+` AS username,
               b.date AS date,
               b.weight_kg AS weightKg,
               b.grip_kg AS gripKg,
               b.bolt_score AS boltScore,
               b.sleep_hours AS sleepHours
        FROM biometric_logs b
        LEFT JOIN users u ON u.id = b.user_id
        LEFT JOIN profiles p ON p.user_id = u.id
        WHERE (COALESCE(b.weight_kg,0) > 0 AND (COALESCE(b.weight_kg,0) < 20 OR COALESCE(b.weight_kg,0) > 300))
           OR COALESCE(b.grip_kg,0) > 100
           OR COALESCE(b.bolt_score,0) > 120
           OR COALESCE(b.sleep_hours,0) > 16
        ORDER BY b.date DESC
        LIMIT 5`)

	// 8. NULL leftovers in columns backfilled by migrations 013/014. The count
	// is the number of NULL values; samples name the table and date.
	nullCountQuery := `SELECT COALESCE(SUM(x.null_count),0) FROM (` + integrityNullUnion + `) x`
	nullSampleQuery := `
        SELECT ` + integrityUsernameExpr + ` AS username,
               x.tbl AS "table",
               x.row_date AS date,
               x.null_count AS nullCount
        FROM (` + integrityNullUnion + `) x
        LEFT JOIN users u ON u.id = x.user_id
        LEFT JOIN profiles p ON p.user_id = u.id
        ORDER BY x.tbl ASC, x.row_date DESC
        LIMIT 5`
	add("null_leftovers", nullCountQuery, nullSampleQuery)

	// 9. Same-day target-history duplicates. Informational: migration 011
	// permits them (check-in + manual edit), but a household may want to know.
	add("duplicate_same_day_snapshots", `
        SELECT COUNT(*) FROM (
          SELECT 1 FROM target_history
          GROUP BY user_id, effective_date
          HAVING COUNT(*) > 1
        )`,
		`
        SELECT `+integrityUsernameExpr+` AS username,
               g.effective_date AS effectiveDate,
               g.snapshot_count AS snapshots
        FROM (
          SELECT user_id, effective_date, COUNT(*) AS snapshot_count
          FROM target_history
          GROUP BY user_id, effective_date
          HAVING COUNT(*) > 1
        ) g
        LEFT JOIN users u ON u.id = g.user_id
        LEFT JOIN profiles p ON p.user_id = u.id
        ORDER BY g.snapshot_count DESC, g.effective_date DESC
        LIMIT 5`)

	// 10. Approved members with no nutrition log in the last 30 days.
	add("stale_members", `
        SELECT COUNT(*) FROM users u
        WHERE COALESCE(u.pending_approval,0) = 0
          AND NOT EXISTS (
            SELECT 1 FROM nutrition_logs n
            WHERE n.user_id = u.id AND n.date >= DATE('now','-30 days')
          )`,
		`
        SELECT COALESCE(NULLIF(TRIM(p.name),''), NULLIF(TRIM(u.username),''), '') AS username,
               u.created_at AS createdAt,
               COALESCE((SELECT MAX(n.date) FROM nutrition_logs n WHERE n.user_id = u.id), '') AS lastNutritionDate
        FROM users u
        LEFT JOIN profiles p ON p.user_id = u.id
        WHERE COALESCE(u.pending_approval,0) = 0
          AND NOT EXISTS (
            SELECT 1 FROM nutrition_logs n
            WHERE n.user_id = u.id AND n.date >= DATE('now','-30 days')
          )
        ORDER BY u.created_at ASC
        LIMIT 5`)

	return findings, errs
}

// integrityNullUnion is shared by the count and sample queries for check 8. It
// walks every column that migrations 013/014 promise is non-NULL after the
// backfill and emits one row per affected record with a NULL-value tally.
const integrityNullUnion = `
    SELECT 'nutrition_logs' AS tbl, n.user_id AS user_id, n.date AS row_date,
           (CASE WHEN n.meal_notes IS NULL THEN 1 ELSE 0 END) AS null_count
    FROM nutrition_logs n
    WHERE n.meal_notes IS NULL
    UNION ALL
    SELECT 'biometric_logs', b.user_id, b.date,
           (CASE WHEN b.weight_kg IS NULL THEN 1 ELSE 0 END
          + CASE WHEN b.waist_cm IS NULL THEN 1 ELSE 0 END
          + CASE WHEN b.grip_kg IS NULL THEN 1 ELSE 0 END
          + CASE WHEN b.bolt_score IS NULL THEN 1 ELSE 0 END
          + CASE WHEN b.sleep_hours IS NULL THEN 1 ELSE 0 END
          + CASE WHEN b.sleep_quality IS NULL THEN 1 ELSE 0 END
          + CASE WHEN b.subjective_feel IS NULL THEN 1 ELSE 0 END
          + CASE WHEN b.body_fat_pct IS NULL THEN 1 ELSE 0 END
          + CASE WHEN b.notes IS NULL THEN 1 ELSE 0 END)
    FROM biometric_logs b
    WHERE b.weight_kg IS NULL OR b.waist_cm IS NULL OR b.grip_kg IS NULL
       OR b.bolt_score IS NULL OR b.sleep_hours IS NULL OR b.sleep_quality IS NULL
       OR b.subjective_feel IS NULL OR b.body_fat_pct IS NULL OR b.notes IS NULL
    UNION ALL
    SELECT 'check_in_logs', c.user_id, c.check_in_date,
           (CASE WHEN c.weight_before IS NULL THEN 1 ELSE 0 END
          + CASE WHEN c.weight_after IS NULL THEN 1 ELSE 0 END
          + CASE WHEN c.calories_before IS NULL THEN 1 ELSE 0 END
          + CASE WHEN c.calories_after IS NULL THEN 1 ELSE 0 END
          + CASE WHEN c.reason IS NULL THEN 1 ELSE 0 END)
    FROM check_in_logs c
    WHERE c.weight_before IS NULL OR c.weight_after IS NULL
       OR c.calories_before IS NULL OR c.calories_after IS NULL OR c.reason IS NULL
    UNION ALL
    SELECT 'body_measurements', m.user_id, m.date,
           (CASE WHEN m.neck_cm IS NULL THEN 1 ELSE 0 END
          + CASE WHEN m.chest_cm IS NULL THEN 1 ELSE 0 END
          + CASE WHEN m.waist_cm IS NULL THEN 1 ELSE 0 END
          + CASE WHEN m.hips_cm IS NULL THEN 1 ELSE 0 END
          + CASE WHEN m.thigh_cm IS NULL THEN 1 ELSE 0 END
          + CASE WHEN m.bicep_cm IS NULL THEN 1 ELSE 0 END
          + CASE WHEN m.shoulders_cm IS NULL THEN 1 ELSE 0 END
          + CASE WHEN m.calves_cm IS NULL THEN 1 ELSE 0 END
          + CASE WHEN m.notes IS NULL THEN 1 ELSE 0 END)
    FROM body_measurements m
    WHERE m.neck_cm IS NULL OR m.chest_cm IS NULL OR m.waist_cm IS NULL
       OR m.hips_cm IS NULL OR m.thigh_cm IS NULL OR m.bicep_cm IS NULL
       OR m.shoulders_cm IS NULL OR m.calves_cm IS NULL OR m.notes IS NULL`

// integrityFinding runs a count query and its bounded sample query.
func (s *SQLiteStore) integrityFinding(ctx context.Context, check, countQuery, sampleQuery string) (IntegrityFinding, error) {
	finding := IntegrityFinding{Check: check, Samples: []map[string]any{}}
	if err := s.db.QueryRowContext(ctx, countQuery).Scan(&finding.Count); err != nil {
		return finding, fmt.Errorf("count: %w", err)
	}
	samples, err := queryIntegritySamples(ctx, s.db, sampleQuery)
	if err != nil {
		return finding, fmt.Errorf("samples: %w", err)
	}
	if samples == nil {
		samples = []map[string]any{}
	}
	finding.Samples = samples
	return finding, nil
}

// integrityDoubledNotes flags rows whose meal_notes repeat a line (>= 12
// characters) or a sentence fragment (>= 20 characters) at least twice.
func (s *SQLiteStore) integrityDoubledNotes(ctx context.Context) (IntegrityFinding, error) {
	finding := IntegrityFinding{Check: "nutrition_doubled_notes", Samples: []map[string]any{}}

	rows, err := s.db.QueryContext(ctx, `
        SELECT `+integrityUsernameExpr+` AS username,
               n.date AS date,
               COALESCE(n.meal_notes,'') AS meal_notes
        FROM nutrition_logs n
        LEFT JOIN users u ON u.id = n.user_id
        LEFT JOIN profiles p ON p.user_id = u.id
        WHERE n.meal_notes IS NOT NULL AND length(TRIM(n.meal_notes)) >= 12
        ORDER BY n.date DESC`)
	if err != nil {
		return finding, fmt.Errorf("query notes: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var username, date, notes string
		if err := rows.Scan(&username, &date, &notes); err != nil {
			return finding, fmt.Errorf("scan notes: %w", err)
		}
		if !hasDoubledNotes(notes) {
			continue
		}
		finding.Count++
		if len(finding.Samples) < 5 {
			finding.Samples = append(finding.Samples, map[string]any{
				"username":  username,
				"date":      date,
				"mealNotes": truncateRunes(notes, 160),
			})
		}
	}
	if err := rows.Err(); err != nil {
		return finding, fmt.Errorf("iterate notes: %w", err)
	}
	return finding, nil
}

// hasDoubledNotes reports whether any normalised line (>= 12 runes) or
// sentence fragment (>= 20 runes) appears twice.
func hasDoubledNotes(notes string) bool {
	seen := make(map[string]struct{})
	for _, line := range strings.Split(notes, "\n") {
		key := normaliseNoteFragment(line)
		if len([]rune(key)) < 12 {
			continue
		}
		if _, dup := seen[key]; dup {
			return true
		}
		seen[key] = struct{}{}
	}

	seen = make(map[string]struct{})
	for _, fragment := range strings.FieldsFunc(notes, func(r rune) bool {
		return r == '.' || r == '!' || r == '?' || r == '\n'
	}) {
		key := normaliseNoteFragment(fragment)
		if len([]rune(key)) < 20 {
			continue
		}
		if _, dup := seen[key]; dup {
			return true
		}
		seen[key] = struct{}{}
	}
	return false
}

// normaliseNoteFragment lowercases and collapses internal whitespace so
// cosmetic spacing does not hide a duplicate.
func normaliseNoteFragment(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// truncateRunes returns value clipped to at most n runes, keeping evidence
// small enough for a report sample.
func truncateRunes(value string, n int) string {
	runes := []rune(value)
	if len(runes) <= n {
		return value
	}
	return string(runes[:n]) + "…"
}

// queryIntegritySamples scans an arbitrary read-only query into JSON-friendly
// maps. Columns are exactly the aliases the check selected, so no schema detail
// leaks beyond the benign evidence columns named in each sample query.
func queryIntegritySamples(ctx context.Context, db *sql.DB, query string, args ...any) ([]map[string]any, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	out := make([]map[string]any, 0, 5)
	for rows.Next() {
		values := make([]any, len(columns))
		targets := make([]any, len(columns))
		for i := range values {
			targets[i] = &values[i]
		}
		if err := rows.Scan(targets...); err != nil {
			return nil, err
		}
		sample := make(map[string]any, len(columns))
		for i, column := range columns {
			if raw, ok := values[i].([]byte); ok {
				sample[column] = string(raw)
				continue
			}
			sample[column] = values[i]
		}
		out = append(out, sample)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
