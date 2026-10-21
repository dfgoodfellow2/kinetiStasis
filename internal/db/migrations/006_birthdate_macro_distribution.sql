-- +goose Up
-- Add birthdate column (YYYY-MM-DD) and macro_distribution column to profiles
-- macro_distribution: 'keto', 'low_carb', 'balanced', 'high_carb'
ALTER TABLE profiles ADD COLUMN birthdate TEXT DEFAULT '';
ALTER TABLE profiles ADD COLUMN macro_distribution TEXT DEFAULT 'balanced';

-- +goose Down
-- SQLite before 3.35 doesn't support DROP COLUMN. If you need to reverse,
-- recreate the table and copy data.
SELECT 1;
