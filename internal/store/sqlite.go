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
               COALESCE(sleep_quality_max,10.0), COALESCE(units,'imperial'), updated_at
        FROM profiles WHERE user_id = ?`, userID,
	).Scan(&p.UserID, &p.Name, &p.Age, &p.Sex, &p.HeightCm, &p.Activity, &p.ExerciseFreq,
		&p.RunningKm, &p.IsLifter, &p.Goal, &p.PrioritizeCarbs, &p.BfPct, &p.HRRest,
		&p.HRMax, &p.GripWeight, &p.TDEELookbackDays, &p.SleepQualityMax, &p.Units, &p.UpdatedAt)
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
          sleep_quality_max,units,updated_at)
        VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
        ON CONFLICT(user_id) DO UPDATE SET
          name=excluded.name, age=excluded.age, sex=excluded.sex, height_cm=excluded.height_cm,
          activity=excluded.activity, exercise_freq=excluded.exercise_freq,
          running_km=excluded.running_km, is_lifter=excluded.is_lifter, goal=excluded.goal,
          prioritize_carbs=excluded.prioritize_carbs, bf_pct=excluded.bf_pct, hr_rest=excluded.hr_rest,
          hr_max=excluded.hr_max, grip_weight=excluded.grip_weight,
          tdee_lookback_days=excluded.tdee_lookback_days, sleep_quality_max=excluded.sleep_quality_max,
          units=excluded.units, updated_at=excluded.updated_at`,
		p.UserID, p.Name, p.Age, p.Sex, p.HeightCm, p.Activity, p.ExerciseFreq, p.RunningKm,
		isLifter, p.Goal, prioritize, p.BfPct, p.HRRest, p.HRMax, p.GripWeight,
		p.TDEELookbackDays, p.SleepQualityMax, p.Units, p.UpdatedAt,
	)
	return err
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

func (s *SQLiteStore) GetLastCheckin(ctx context.Context, userID string) (models.CheckInLog, error) {
	var out models.CheckInLog
	err := s.db.QueryRowContext(ctx, `SELECT id, user_id, check_in_date, weight_before, weight_after, calories_before, calories_after, reason, created_at FROM check_in_logs WHERE user_id = ? ORDER BY check_in_date DESC LIMIT 1`, userID).Scan(&out.ID, &out.UserID, &out.CheckInDate, &out.WeightBefore, &out.WeightAfter, &out.CaloriesBefore, &out.CaloriesAfter, &out.Reason, &out.CreatedAt)
	return out, err
}

// CreateCheckinLog inserts a new check-in row.
func (s *SQLiteStore) CreateCheckinLog(ctx context.Context, c *models.CheckInLog) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO check_in_logs (id, user_id, check_in_date, weight_before, weight_after, calories_before, calories_after, reason, created_at) VALUES (?,?,?,?,?,?,?,?,?)`, c.ID, c.UserID, c.CheckInDate, c.WeightBefore, c.WeightAfter, c.CaloriesBefore, c.CaloriesAfter, c.Reason, c.CreatedAt)
	return err
}

// FindUserByUsername returns a user by username.
func (s *SQLiteStore) FindUserByUsername(ctx context.Context, username string) (models.User, error) {
	var u models.User
	var isAdmin int
	err := s.db.QueryRowContext(ctx, `SELECT id, username, email, is_admin, created_at, updated_at FROM users WHERE username = ? LIMIT 1`, username).Scan(&u.ID, &u.Username, &u.Email, &isAdmin, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return u, err
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

// PingContext implements Store.PingContext
func (s *SQLiteStore) PingContext(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// --- UserStore implementations ---
func (s *SQLiteStore) CountUsers(ctx context.Context) (int, error) {
	var cnt int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&cnt); err != nil {
		return 0, err
	}
	return cnt, nil
}

func (s *SQLiteStore) CreateUser(ctx context.Context, id, username, email, passwordHash string, isAdmin bool, createdAt, updatedAt string) error {
	admin := 0
	if isAdmin {
		admin = 1
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO users (id, username, email, password, is_admin, created_at, updated_at) VALUES (?,?,?,?,?,?,?)`, id, username, email, passwordHash, admin, createdAt, updatedAt)
	return err
}

func (s *SQLiteStore) FindUserByLogin(ctx context.Context, login string) (id, username, passwordHash string, isAdmin bool, err error) {
	var adminInt int
	err = s.db.QueryRowContext(ctx, `SELECT id, username, password, is_admin FROM users WHERE username = ? OR email = ? LIMIT 1`, login, login).Scan(&id, &username, &passwordHash, &adminInt)
	if err != nil {
		return "", "", "", false, err
	}
	return id, username, passwordHash, adminInt == 1, nil
}

func (s *SQLiteStore) GetUserByID(ctx context.Context, userID string) (models.User, error) {
	var u models.User
	var isAdmin int
	if err := s.db.QueryRowContext(ctx, `SELECT id, username, email, is_admin, created_at, updated_at FROM users WHERE id = ?`, userID).Scan(&u.ID, &u.Username, &u.Email, &isAdmin, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return u, err
	}
	u.IsAdmin = isAdmin == 1
	return u, nil
}

func (s *SQLiteStore) SaveRefreshToken(ctx context.Context, id, userID, tokenHash, expiresAt, createdAt string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, created_at) VALUES (?,?,?,?,?)`, id, userID, tokenHash, expiresAt, createdAt)
	return err
}

func (s *SQLiteStore) FindRefreshToken(ctx context.Context, tokenHash string) (userID string, isAdmin bool, expiresAt string, err error) {
	var adminInt int
	err = s.db.QueryRowContext(ctx, `SELECT u.id, u.is_admin, rt.expires_at FROM refresh_tokens rt JOIN users u ON u.id = rt.user_id WHERE rt.token_hash = ?`, tokenHash).Scan(&userID, &adminInt, &expiresAt)
	if err != nil {
		return "", false, "", err
	}
	return userID, adminInt == 1, expiresAt, nil
}

func (s *SQLiteStore) DeleteRefreshToken(ctx context.Context, tokenHash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE token_hash = ?`, tokenHash)
	return err
}

func (s *SQLiteStore) ListUsers(ctx context.Context) ([]models.User, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, username, email, is_admin, created_at, updated_at FROM users ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.User
	for rows.Next() {
		var u models.User
		var isAdmin int
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &isAdmin, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		u.IsAdmin = isAdmin == 1
		out = append(out, u)
	}
	return out, nil
}

func (s *SQLiteStore) DeleteUser(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, userID)
	return err
}

func (s *SQLiteStore) PromoteUser(ctx context.Context, userID, now string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET is_admin = 1, updated_at = ? WHERE id = ?`, now, userID)
	return err
}

func (s *SQLiteStore) DemoteUser(ctx context.Context, userID, now string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET is_admin = 0, updated_at = ? WHERE id = ?`, now, userID)
	return err
}

func (s *SQLiteStore) CountAdmins(ctx context.Context) (int, error) {
	var cnt int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE is_admin = 1`).Scan(&cnt); err != nil {
		return 0, err
	}
	return cnt, nil
}

// --- Nutrition ---
func (s *SQLiteStore) FetchNutritionLogsRange(ctx context.Context, userID, from, to string) ([]models.NutritionLog, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,user_id,date,calories,protein_g,carbs_g,fat_g,fiber_g,water_ml,meal_notes,updated_at FROM nutrition_logs WHERE user_id = ? AND date >= ? AND date <= ? ORDER BY date ASC`, userID, from, to)
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

func (s *SQLiteStore) GetNutritionLog(ctx context.Context, userID, date string) (models.NutritionLog, error) {
	var n models.NutritionLog
	err := s.db.QueryRowContext(ctx, `SELECT id,user_id,date,calories,protein_g,carbs_g,fat_g,fiber_g,water_ml,meal_notes,updated_at FROM nutrition_logs WHERE user_id = ? AND date = ?`, userID, date).Scan(&n.ID, &n.UserID, &n.Date, &n.Calories, &n.ProteinG, &n.CarbsG, &n.FatG, &n.FiberG, &n.WaterMl, &n.MealNotes, &n.UpdatedAt)
	return n, err
}

func (s *SQLiteStore) CreateNutritionLog(ctx context.Context, n *models.NutritionLog) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO nutrition_logs (id,user_id,date,calories,protein_g,carbs_g,fat_g,fiber_g,water_ml,meal_notes,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)`, n.ID, n.UserID, n.Date, n.Calories, n.ProteinG, n.CarbsG, n.FatG, n.FiberG, n.WaterMl, n.MealNotes, n.UpdatedAt)
	return err
}

func (s *SQLiteStore) UpdateNutritionLog(ctx context.Context, n *models.NutritionLog) error {
	_, err := s.db.ExecContext(ctx, `UPDATE nutrition_logs SET calories = ?, protein_g = ?, carbs_g = ?, fat_g = ?, fiber_g = ?, water_ml = ?, meal_notes = ?, updated_at = ? WHERE user_id = ? AND date = ?`, n.Calories, n.ProteinG, n.CarbsG, n.FatG, n.FiberG, n.WaterMl, n.MealNotes, n.UpdatedAt, n.UserID, n.Date)
	return err
}

func (s *SQLiteStore) DeleteNutritionLog(ctx context.Context, userID, date string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM nutrition_logs WHERE user_id = ? AND date = ?`, userID, date)
	return err
}

// --- Biometric ---
func (s *SQLiteStore) FetchBiometricLogsRange(ctx context.Context, userID, from, to string) ([]models.BiometricLog, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,user_id,date,weight_kg,waist_cm,grip_kg,bolt_score,sleep_hours,sleep_quality,subjective_feel,body_fat_pct,notes,updated_at FROM biometric_logs WHERE user_id = ? AND date >= ? AND date <= ? ORDER BY date ASC`, userID, from, to)
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

func (s *SQLiteStore) GetBiometricLog(ctx context.Context, userID, date string) (models.BiometricLog, error) {
	var b models.BiometricLog
	err := s.db.QueryRowContext(ctx, `SELECT id,user_id,date,weight_kg,waist_cm,grip_kg,bolt_score,sleep_hours,sleep_quality,subjective_feel,body_fat_pct,notes,updated_at FROM biometric_logs WHERE user_id = ? AND date = ?`, userID, date).Scan(&b.ID, &b.UserID, &b.Date, &b.WeightKg, &b.WaistCm, &b.GripKg, &b.BoltScore, &b.SleepHours, &b.SleepQuality, &b.SubjectiveFeel, &b.BodyFatPct, &b.Notes, &b.UpdatedAt)
	return b, err
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
	return err
}

func (s *SQLiteStore) UpdateBiometricLog(ctx context.Context, b *models.BiometricLog) error {
	_, err := s.db.ExecContext(ctx, `UPDATE biometric_logs SET weight_kg=?,waist_cm=?,grip_kg=?,bolt_score=?,sleep_hours=?,sleep_quality=?,subjective_feel=?,body_fat_pct=?,notes=?,updated_at=? WHERE user_id=? AND date = ?`, b.WeightKg, b.WaistCm, b.GripKg, b.BoltScore, b.SleepHours, b.SleepQuality, b.SubjectiveFeel, b.BodyFatPct, b.Notes, b.UpdatedAt, b.UserID, b.Date)
	return err
}

func (s *SQLiteStore) DeleteBiometricLog(ctx context.Context, userID, date string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM biometric_logs WHERE user_id = ? AND date = ?`, userID, date)
	return err
}

// --- Workouts ---
func (s *SQLiteStore) FetchWorkoutsRange(ctx context.Context, userID, from, to string) ([]models.WorkoutEntry, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, user_id, date, slot, COALESCE(title,''), COALESCE(raw_notes,''), COALESCE(duration_min,0), COALESCE(calories_burned,0), COALESCE(mwv,0), COALESCE(nds,0), COALESCE(session_density,0), COALESCE(exercises_json,'[]'), COALESCE(metadata_json,'{}'), updated_at FROM workouts WHERE user_id = ? AND date >= ? AND date <= ? ORDER BY date ASC, slot ASC`, userID, from, to)
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

func (s *SQLiteStore) GetWorkout(ctx context.Context, userID, date, slot string) (models.WorkoutEntry, error) {
	var w models.WorkoutEntry
	var exercisesJSON, metadataJSON string
	err := s.db.QueryRowContext(ctx, `SELECT id, user_id, date, slot, COALESCE(title,''), COALESCE(raw_notes,''), COALESCE(duration_min,0), COALESCE(calories_burned,0), COALESCE(mwv,0), COALESCE(nds,0), COALESCE(session_density,0), COALESCE(exercises_json,'[]'), COALESCE(metadata_json,'{}'), updated_at FROM workouts WHERE user_id = ? AND date = ? AND slot = ?`, userID, date, slot).Scan(&w.ID, &w.UserID, &w.Date, &w.Slot, &w.Title, &w.RawNotes, &w.DurationMin, &w.CaloriesBurned, &w.MWV, &w.NDS, &w.SessionDensity, &exercisesJSON, &metadataJSON, &w.UpdatedAt)
	if err != nil {
		return w, err
	}
	if exercisesJSON != "" {
		_ = json.Unmarshal([]byte(exercisesJSON), &w.Exercises)
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
		return err
	}
	if ra, _ := res.RowsAffected(); ra == 0 {
		_, err = s.db.ExecContext(ctx, `INSERT INTO workouts (id,user_id,date,slot,title,raw_notes,duration_min,calories_burned,mwv,nds,session_density,exercises_json,metadata_json,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, w.ID, w.UserID, w.Date, w.Slot, w.Title, w.RawNotes, w.DurationMin, w.CaloriesBurned, w.MWV, w.NDS, w.SessionDensity, string(exb), string(metb), w.UpdatedAt)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *SQLiteStore) UpdateWorkout(ctx context.Context, w *models.WorkoutEntry) (models.WorkoutEntry, error) {
	exb, _ := json.Marshal(w.Exercises)
	metb, _ := json.Marshal(w.Metadata)
	_, err := s.db.ExecContext(ctx, `UPDATE workouts SET title=?,raw_notes=?,duration_min=?,calories_burned=?,mwv=?,nds=?,session_density=?,exercises_json=?,metadata_json=?,updated_at=? WHERE user_id=? AND date=? AND slot=?`, w.Title, w.RawNotes, w.DurationMin, w.CaloriesBurned, w.MWV, w.NDS, w.SessionDensity, string(exb), string(metb), w.UpdatedAt, w.UserID, w.Date, w.Slot)
	if err != nil {
		return models.WorkoutEntry{}, err
	}
	out, err := s.GetWorkout(ctx, w.UserID, w.Date, w.Slot)
	return out, err
}

func (s *SQLiteStore) DeleteWorkout(ctx context.Context, userID, date, slot string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM workouts WHERE user_id = ? AND date = ? AND slot = ?`, userID, date, slot)
	return err
}

// --- Targets ---
func (s *SQLiteStore) UpsertTargets(ctx context.Context, t *models.Targets) error {
	eatBack := 0
	if t.EatBackExercise {
		eatBack = 1
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO targets (user_id,calories,protein_g,carbs_g,fat_g,fiber_g,water_ml,eat_back_exercise,updated_at) VALUES (?,?,?,?,?,?,?,?,?) ON CONFLICT(user_id) DO UPDATE SET calories=excluded.calories,protein_g=excluded.protein_g,carbs_g=excluded.carbs_g,fat_g=excluded.fat_g,fiber_g=excluded.fiber_g,water_ml=excluded.water_ml,eat_back_exercise=excluded.eat_back_exercise,updated_at=excluded.updated_at`, t.UserID, t.Calories, t.ProteinG, t.CarbsG, t.FatG, t.FiberG, t.WaterMl, eatBack, t.UpdatedAt)
	return err
}

func (s *SQLiteStore) CreateTargetSnapshot(ctx context.Context, snap *models.TargetSnapshot) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO target_history (id,user_id,effective_date,calories,protein_g,carbs_g,fat_g,fiber_g,created_at) VALUES (?,?,?,?,?,?,?,?,?)`, snap.ID, snap.UserID, snap.EffectiveDate, snap.Calories, snap.ProteinG, snap.CarbsG, snap.FatG, snap.FiberG, snap.CreatedAt)
	return err
}

func (s *SQLiteStore) FetchTargetHistory(ctx context.Context, userID string) ([]models.TargetSnapshot, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,user_id,effective_date,calories,protein_g,carbs_g,fat_g,fiber_g,created_at FROM target_history WHERE user_id = ? ORDER BY effective_date DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.TargetSnapshot
	for rows.Next() {
		var t models.TargetSnapshot
		if err := rows.Scan(&t.ID, &t.UserID, &t.EffectiveDate, &t.Calories, &t.ProteinG, &t.CarbsG, &t.FatG, &t.FiberG, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

// --- Meals ---
func (s *SQLiteStore) FetchSavedMeals(ctx context.Context, userID string) ([]models.SavedMeal, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,user_id,name,calories,protein_g,carbs_g,fat_g,fiber_g,created_at,updated_at FROM saved_meals WHERE user_id = ? ORDER BY name ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.SavedMeal
	for rows.Next() {
		var m models.SavedMeal
		if err := rows.Scan(&m.ID, &m.UserID, &m.Name, &m.Calories, &m.ProteinG, &m.CarbsG, &m.FatG, &m.FiberG, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

func (s *SQLiteStore) CreateSavedMeal(ctx context.Context, m *models.SavedMeal) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO saved_meals (id,user_id,name,calories,protein_g,carbs_g,fat_g,fiber_g,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?)`, m.ID, m.UserID, m.Name, m.Calories, m.ProteinG, m.CarbsG, m.FatG, m.FiberG, m.CreatedAt, m.UpdatedAt)
	return err
}

func (s *SQLiteStore) DeleteSavedMeal(ctx context.Context, userID, mealID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM saved_meals WHERE id = ? AND user_id = ?`, mealID, userID)
	return err
}

func (s *SQLiteStore) FetchMealTemplates(ctx context.Context, userID string) ([]models.MealTemplate, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,user_id,name,meals_json,created_at,updated_at FROM meal_templates WHERE user_id = ? ORDER BY name ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.MealTemplate
	for rows.Next() {
		var id, uid, name, mealsJSON, createdAt, updatedAt string
		if err := rows.Scan(&id, &uid, &name, &mealsJSON, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		var meals []models.SavedMeal
		if mealsJSON != "" {
			if err := json.Unmarshal([]byte(mealsJSON), &meals); err != nil {
				slog.Warn("unmarshal meals_json failed", "err", err)
			}
		}
		out = append(out, models.MealTemplate{ID: id, UserID: uid, Name: name, Meals: meals, CreatedAt: createdAt, UpdatedAt: updatedAt})
	}
	return out, nil
}

func (s *SQLiteStore) CreateMealTemplate(ctx context.Context, t *models.MealTemplate) error {
	b, _ := json.Marshal(t.Meals)
	_, err := s.db.ExecContext(ctx, `INSERT INTO meal_templates (id,user_id,name,meals_json,created_at,updated_at) VALUES (?,?,?,?,?,?)`, t.ID, t.UserID, t.Name, string(b), t.CreatedAt, t.UpdatedAt)
	return err
}

func (s *SQLiteStore) DeleteMealTemplate(ctx context.Context, userID, templateID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM meal_templates WHERE id = ? AND user_id = ?`, templateID, userID)
	return err
}

// --- Measurements ---
func (s *SQLiteStore) FetchMeasurementsRange(ctx context.Context, userID, from, to string) ([]models.BodyMeasurement, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,user_id,date,neck_cm,chest_cm,waist_cm,hips_cm,thigh_cm,bicep_cm,notes,created_at,shoulders_cm,calves_cm FROM body_measurements WHERE user_id = ? AND date >= ? AND date <= ? ORDER BY date ASC`, userID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.BodyMeasurement
	for rows.Next() {
		var m models.BodyMeasurement
		if err := rows.Scan(&m.ID, &m.UserID, &m.Date, &m.NeckCm, &m.ChestCm, &m.WaistCm, &m.HipsCm, &m.ThighCm, &m.BicepCm, &m.Notes, &m.CreatedAt, &m.ShouldersCm, &m.CalvesCm); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

func (s *SQLiteStore) CreateMeasurement(ctx context.Context, m *models.BodyMeasurement) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO body_measurements (id,user_id,date,neck_cm,chest_cm,waist_cm,hips_cm,thigh_cm,bicep_cm,notes,created_at,shoulders_cm,calves_cm) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(user_id,date) DO UPDATE SET neck_cm=excluded.neck_cm,chest_cm=excluded.chest_cm,waist_cm=excluded.waist_cm,hips_cm=excluded.hips_cm,thigh_cm=excluded.thigh_cm,bicep_cm=excluded.bicep_cm,notes=excluded.notes,shoulders_cm=excluded.shoulders_cm,calves_cm=excluded.calves_cm`, m.ID, m.UserID, m.Date, m.NeckCm, m.ChestCm, m.WaistCm, m.HipsCm, m.ThighCm, m.BicepCm, m.Notes, m.CreatedAt, m.ShouldersCm, m.CalvesCm)
	return err
}

func (s *SQLiteStore) UpdateMeasurement(ctx context.Context, m *models.BodyMeasurement) error {
	_, err := s.db.ExecContext(ctx, `UPDATE body_measurements SET neck_cm=?,chest_cm=?,waist_cm=?,hips_cm=?,thigh_cm=?,bicep_cm=?,notes=?,shoulders_cm=?,calves_cm=? WHERE user_id=? AND date=?`, m.NeckCm, m.ChestCm, m.WaistCm, m.HipsCm, m.ThighCm, m.BicepCm, m.Notes, m.ShouldersCm, m.CalvesCm, m.UserID, m.Date)
	return err
}

func (s *SQLiteStore) DeleteMeasurement(ctx context.Context, userID, date string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM body_measurements WHERE user_id = ? AND date = ?`, userID, date)
	return err
}

func (s *SQLiteStore) FetchBodyMeasurementsRangeMap(ctx context.Context, userID, from, to string) (map[string]float64, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT date, COALESCE(waist_cm,0) FROM body_measurements WHERE user_id = ? AND date >= ? AND date <= ?`, userID, from, to)
	if err != nil {
		return nil, err
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
