-- +goose Up
-- Persist the adaptive-TDEE snapshot that produced each check-in so that
-- history can show exactly what the engine returned at that time.
-- Types mirror the existing check_in_logs columns: REAL for the TDEE value,
-- TEXT for the confidence tier and INTEGER for the lookback window.
ALTER TABLE check_in_logs ADD COLUMN observed_tdee REAL NOT NULL DEFAULT 0;
ALTER TABLE check_in_logs ADD COLUMN tdee_confidence TEXT NOT NULL DEFAULT '';
ALTER TABLE check_in_logs ADD COLUMN lookback_days INTEGER NOT NULL DEFAULT 0;

-- +goose Down
-- modernc.org/sqlite v1.29.10 bundles SQLite 3.45.3, which supports DROP COLUMN.
ALTER TABLE check_in_logs DROP COLUMN observed_tdee;
ALTER TABLE check_in_logs DROP COLUMN tdee_confidence;
ALTER TABLE check_in_logs DROP COLUMN lookback_days;
