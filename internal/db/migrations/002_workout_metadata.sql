-- +goose Up
ALTER TABLE workouts ADD COLUMN metadata_json TEXT NOT NULL DEFAULT '{}';

-- +goose Down
-- SQLite does not support DROP COLUMN before 3.35; leave as no-op
SELECT 1;
