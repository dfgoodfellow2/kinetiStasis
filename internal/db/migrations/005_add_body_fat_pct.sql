-- +goose Up
-- Add missing body_fat_pct column to biometric_logs
-- NOTE: SQLite supports ALTER TABLE ... ADD COLUMN; this will be a no-op if run against a DB
-- that already has the column only if the migration runner skips already-applied migrations.
-- If you need true idempotency at the SQL level, you'd recreate the table and copy data
-- (more complex and not necessary for a simple additive column).
ALTER TABLE biometric_logs ADD COLUMN body_fat_pct REAL DEFAULT 0;

-- +goose Down
-- SQLite doesn't reliably support DROP COLUMN in older versions. To reverse this
-- you'd need to recreate the table without the column and copy data. Marking as
-- irreversible here.
-- Irreversible
