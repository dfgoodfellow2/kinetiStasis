-- +goose Up
ALTER TABLE body_measurements ADD COLUMN shoulders_cm REAL;
ALTER TABLE body_measurements ADD COLUMN calves_cm REAL;

-- +goose Down
ALTER TABLE body_measurements DROP COLUMN shoulders_cm;
ALTER TABLE body_measurements DROP COLUMN calves_cm;
