-- +goose Up
-- The original target_history table only allowed one row per user per day
-- (UNIQUE(user_id, effective_date)). A check-in and a manual target edit on the
-- same day therefore collided and the second write failed, losing history.
-- Rebuild the table so the uniqueness key includes created_at, while keeping a
-- non-unique (user_id, effective_date) index for the history read.
--
-- Column list/types are identical to 001_init + 007: the rebuild must not
-- change how water_ml or eat_back_exercise round-trip.
-- +goose StatementBegin
CREATE TABLE target_history_new (
    id                TEXT PRIMARY KEY,
    user_id           TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    effective_date    TEXT NOT NULL,
    calories          REAL NOT NULL,
    protein_g         REAL NOT NULL,
    carbs_g           REAL NOT NULL,
    fat_g             REAL NOT NULL,
    fiber_g           REAL NOT NULL,
    water_ml          REAL NOT NULL DEFAULT 0,
    eat_back_exercise INTEGER NOT NULL DEFAULT 0,
    created_at        TEXT NOT NULL,
    UNIQUE(user_id, effective_date, created_at)
);

INSERT INTO target_history_new (
    id, user_id, effective_date, calories, protein_g, carbs_g, fat_g, fiber_g,
    water_ml, eat_back_exercise, created_at
)
SELECT
    id, user_id, effective_date, calories, protein_g, carbs_g, fat_g, fiber_g,
    water_ml, eat_back_exercise, created_at
FROM target_history;

DROP TABLE target_history;
ALTER TABLE target_history_new RENAME TO target_history;

CREATE INDEX IF NOT EXISTS idx_target_history_user_date
    ON target_history(user_id, effective_date DESC);
-- +goose StatementEnd

-- +goose Down
-- Best-effort no-op. Reversing this migration requires the same table rebuild
-- in reverse, and it would fail (or silently drop history rows) if any user has
-- two same-day snapshots created after the fix. The Down path is not exposed;
-- restore from a backup or run a deliberate, reviewed rebuild instead.
