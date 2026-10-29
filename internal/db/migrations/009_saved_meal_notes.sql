-- +goose Up
-- Saved meals capture the meal description the user typed, so applying a
-- saved meal (or a template entry built from one) restores the notes as well
-- as the macros. Type mirrors nutrition_logs.meal_notes (TEXT); legacy rows
-- default to an empty description.
ALTER TABLE saved_meals ADD COLUMN notes TEXT NOT NULL DEFAULT '';

-- +goose Down
-- modernc.org/sqlite v1.29.10 bundles SQLite 3.45.3, which supports DROP COLUMN.
ALTER TABLE saved_meals DROP COLUMN notes;
