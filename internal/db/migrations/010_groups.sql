-- +goose Up
-- Groups let a small set of users share selected signals (clock/logging,
-- streaks and adherence) and templates. IDs follow the repo convention of
-- UUID strings stored as TEXT; timestamps are RFC3339 TEXT.
CREATE TABLE IF NOT EXISTS groups (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    created_by  TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

-- Membership also carries the per-member sharing toggles. Toggles default to
-- on so a freshly joined member contributes the standard card.
CREATE TABLE IF NOT EXISTS group_members (
    group_id        TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role            TEXT NOT NULL DEFAULT 'member',
    share_clock     INTEGER NOT NULL DEFAULT 1,
    share_streak    INTEGER NOT NULL DEFAULT 1,
    share_adherence INTEGER NOT NULL DEFAULT 1,
    joined_at       TEXT NOT NULL,
    PRIMARY KEY (group_id, user_id)
);

-- Invite codes are single-use and expire after seven days. used_by is empty
-- until the code is consumed; keeping the row preserves an audit trail.
CREATE TABLE IF NOT EXISTS invites (
    code        TEXT PRIMARY KEY,
    group_id    TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    created_by  TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at  TEXT NOT NULL,
    used_by     TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL
);

-- An empty group_id means the template is private to its owner. There is no
-- FK here because ALTER TABLE cannot add one to an existing table in SQLite;
-- group deletion explicitly unshares templates in the store layer.
ALTER TABLE meal_templates ADD COLUMN group_id TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_group_members_user ON group_members(user_id);
CREATE INDEX IF NOT EXISTS idx_invites_group       ON invites(group_id);
CREATE INDEX IF NOT EXISTS idx_meal_templates_group ON meal_templates(group_id);

-- +goose Down
-- modernc.org/sqlite v1.29.10 bundles SQLite 3.45.3, which supports DROP COLUMN.
DROP INDEX IF EXISTS idx_meal_templates_group;
DROP INDEX IF EXISTS idx_invites_group;
DROP INDEX IF EXISTS idx_group_members_user;
ALTER TABLE meal_templates DROP COLUMN group_id;
DROP TABLE IF EXISTS invites;
DROP TABLE IF EXISTS group_members;
DROP TABLE IF EXISTS groups;
