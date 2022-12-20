-- +migrate Up
ALTER TABLE users ADD COLUMN last_seen_public TINYINT(1) NOT NULL DEFAULT 1;

-- +migrate Down
