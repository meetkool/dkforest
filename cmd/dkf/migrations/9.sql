-- +migrate Up
ALTER TABLE users ADD COLUMN refresh_rate INTEGER NOT NULL DEFAULT 5;

-- +migrate Down
