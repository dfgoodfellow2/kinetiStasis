-- +goose Up
-- Opt-in TOTP two-factor authentication.
--
-- Nothing in this migration force-enrols anyone: users.totp_secret defaults to
-- the empty string, and a non-empty value only appears after a live code has
-- confirmed possession of the authenticator. The companion tables store only
-- SHA-256 hex hashes of secrets that are handed to the client once:
--
--   recovery_codes   single-use 12-char backup codes (used_at == '' means live)
--   login_challenges short-lived (5 min) single-use two-step login stubs
--   trusted_devices  30-day remember-device hashes, revoked on disable/reset
--
-- +goose StatementBegin
ALTER TABLE users ADD COLUMN totp_secret TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS recovery_codes (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash  TEXT NOT NULL,
    used_at    TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS login_challenges (
    challenge_hash TEXT PRIMARY KEY,
    user_id        TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at     TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS trusted_devices (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_hash  TEXT NOT NULL,
    created_at   TEXT NOT NULL,
    last_used_at TEXT NOT NULL,
    expires_at   TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_recovery_codes_user
    ON recovery_codes(user_id);
CREATE INDEX IF NOT EXISTS idx_login_challenges_user
    ON login_challenges(user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_trusted_devices_hash
    ON trusted_devices(device_hash);
CREATE INDEX IF NOT EXISTS idx_trusted_devices_user
    ON trusted_devices(user_id);
-- +goose StatementEnd

-- +goose Down
-- Deliberately a no-op. Dropping the companion tables would silently destroy
-- every enrolled authenticator, recovery code, pending login and remembered
-- device. SQLite also cannot drop the users.totp_secret column without a table
-- rebuild, and a Down migration must never turn 2FA off behind an operator's
-- back. Restore from a backup if a rollback is truly required.
