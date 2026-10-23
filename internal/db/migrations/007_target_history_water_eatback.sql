-- +goose Up
-- Persist water and eat-back settings in target snapshots so that
-- water-only or eat-back-only target edits are represented in history.
-- Types mirror targets.water_ml (REAL) and targets.eat_back_exercise (INTEGER).
ALTER TABLE target_history ADD COLUMN water_ml REAL NOT NULL DEFAULT 0;
ALTER TABLE target_history ADD COLUMN eat_back_exercise INTEGER NOT NULL DEFAULT 0;

-- +goose Down
-- modernc.org/sqlite v1.29.10 bundles SQLite 3.45.3, which supports DROP COLUMN.
ALTER TABLE target_history DROP COLUMN water_ml;
ALTER TABLE target_history DROP COLUMN eat_back_exercise;
