-- +goose Up
-- Registration gate. The default is the closed posture: invite-only sign-ups
-- for any instance that already has a user. A zero-user instance always allows
-- the first registration (that user becomes the admin/owner), regardless of
-- the mode stored here.
--
-- Registration invites are single-use admin-issued codes, mirroring the group
-- invite contract (7-day expiry, no-ambiguous alphabet). used_by is recorded
-- for audit and intentionally has no foreign key: the row survives even if the
-- invited account is later removed.
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS registration_invites (
    code       TEXT PRIMARY KEY,
    created_by TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    used_by    TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS instance_settings (
    "key" TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

INSERT INTO instance_settings ("key", value) VALUES ('registration_mode', 'invite_only');

-- Pending accounts belong to the public_approval mode. They exist as user rows
-- (so an admin can review the request) but Login refuses to issue tokens until
-- the row is approved.
ALTER TABLE users ADD COLUMN pending_approval INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_registration_invites_created
    ON registration_invites(created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- Deliberately a no-op. Dropping instance_settings or registration_invites
-- would discard the configured registration posture and the invite audit
-- trail, and SQLite cannot meaningfully reverse the users column addition
-- without rebuilding the table. Restore from a backup instead.
