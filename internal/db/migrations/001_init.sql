-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS users (
    id          TEXT PRIMARY KEY,
    username    TEXT UNIQUE NOT NULL,
    email       TEXT UNIQUE NOT NULL,
    password    TEXT NOT NULL,
    is_admin    INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS refresh_tokens (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  TEXT UNIQUE NOT NULL,
    expires_at  TEXT NOT NULL,
    created_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS profiles (
    user_id             TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    name                TEXT,
    age                 INTEGER,
    sex                 TEXT,
    height_cm           REAL,
    activity            TEXT,
    exercise_freq       INTEGER,
    running_km       REAL,
    is_lifter           INTEGER DEFAULT 0,
    goal                TEXT,
    prioritize_carbs    INTEGER DEFAULT 0,
    bf_pct              REAL,
    hr_rest             INTEGER,
    hr_max              INTEGER,
    grip_weight         REAL DEFAULT 0.5,
    tdee_lookback_days  INTEGER DEFAULT 90,
    sleep_quality_max   REAL DEFAULT 10.0,
    units               TEXT DEFAULT 'imperial',
    updated_at          TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS nutrition_logs (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    date        TEXT NOT NULL,
    calories    REAL NOT NULL DEFAULT 0,
    protein_g   REAL NOT NULL DEFAULT 0,
    carbs_g     REAL NOT NULL DEFAULT 0,
    fat_g       REAL NOT NULL DEFAULT 0,
    fiber_g     REAL NOT NULL DEFAULT 0,
    water_ml    REAL NOT NULL DEFAULT 0,
    meal_notes  TEXT,
    updated_at  TEXT NOT NULL,
    UNIQUE(user_id, date)
);

CREATE TABLE IF NOT EXISTS biometric_logs (
    id              TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    date            TEXT NOT NULL,
    weight_kg      REAL,
    waist_cm        REAL,
    grip_kg         REAL,
    bolt_score      REAL,
    sleep_hours     REAL,
    sleep_quality   REAL,
    subjective_feel INTEGER,
    notes           TEXT,
    updated_at      TEXT NOT NULL,
    UNIQUE(user_id, date)
);

CREATE TABLE IF NOT EXISTS workouts (
    id              TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    date            TEXT NOT NULL,
    slot            TEXT NOT NULL,
    title           TEXT,
    raw_notes       TEXT,
    duration_min    REAL,
    calories_burned REAL,
    mwv             REAL,
    nds             REAL,
    session_density REAL,
    exercises_json  TEXT NOT NULL DEFAULT '[]',
    updated_at      TEXT NOT NULL,
    UNIQUE(user_id, date, slot)
);

CREATE TABLE IF NOT EXISTS targets (
    user_id         TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    calories        REAL NOT NULL DEFAULT 2000,
    protein_g       REAL NOT NULL DEFAULT 150,
    carbs_g         REAL NOT NULL DEFAULT 200,
    fat_g           REAL NOT NULL DEFAULT 67,
    fiber_g         REAL NOT NULL DEFAULT 30,
    water_ml        REAL NOT NULL DEFAULT 2500,
    eat_back_exercise INTEGER DEFAULT 0,
    updated_at      TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS target_history (
    id              TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    effective_date  TEXT NOT NULL,
    calories        REAL NOT NULL,
    protein_g       REAL NOT NULL,
    carbs_g         REAL NOT NULL,
    fat_g           REAL NOT NULL,
    fiber_g         REAL NOT NULL,
    created_at      TEXT NOT NULL,
    UNIQUE(user_id, effective_date)
);

CREATE TABLE IF NOT EXISTS saved_meals (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    calories    REAL NOT NULL DEFAULT 0,
    protein_g   REAL NOT NULL DEFAULT 0,
    carbs_g     REAL NOT NULL DEFAULT 0,
    fat_g       REAL NOT NULL DEFAULT 0,
    fiber_g     REAL NOT NULL DEFAULT 0,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS meal_templates (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    meals_json  TEXT NOT NULL DEFAULT '[]',
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS body_measurements (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    date        TEXT NOT NULL,
    neck_cm     REAL,
    chest_cm    REAL,
    waist_cm    REAL,
    hips_cm     REAL,
    thigh_cm    REAL,
    bicep_cm    REAL,
    notes       TEXT,
    created_at  TEXT NOT NULL,
    UNIQUE(user_id, date)
);

CREATE INDEX IF NOT EXISTS idx_nutrition_logs_user_date    ON nutrition_logs(user_id, date DESC);
CREATE INDEX IF NOT EXISTS idx_biometric_logs_user_date   ON biometric_logs(user_id, date DESC);
CREATE INDEX IF NOT EXISTS idx_workouts_user_date         ON workouts(user_id, date DESC);
CREATE INDEX IF NOT EXISTS idx_target_history_user_date   ON target_history(user_id, effective_date DESC);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user        ON refresh_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_saved_meals_user           ON saved_meals(user_id);
CREATE INDEX IF NOT EXISTS idx_meal_templates_user        ON meal_templates(user_id);
CREATE INDEX IF NOT EXISTS idx_body_measurements_user     ON body_measurements(user_id, date DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS body_measurements;
DROP TABLE IF EXISTS meal_templates;
DROP TABLE IF EXISTS saved_meals;
DROP TABLE IF EXISTS target_history;
DROP TABLE IF EXISTS targets;
DROP TABLE IF EXISTS workouts;
DROP TABLE IF EXISTS biometric_logs;
DROP TABLE IF EXISTS nutrition_logs;
DROP TABLE IF EXISTS profiles;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
