-- +migrate Up
ALTER TABLE users ADD COLUMN last_seen_at DATETIME NOT NULL DEFAULT 0;

-- +migrate Down
