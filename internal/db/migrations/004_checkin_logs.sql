-- +goose Up
-- +goose StatementBegin
CREATE TABLE check_in_logs (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    check_in_date TEXT NOT NULL,
    weight_before REAL,
    weight_after REAL,
    calories_before INTEGER,
    calories_after INTEGER,
    reason TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(user_id, check_in_date)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS check_in_logs;
-- +goose StatementEnd
