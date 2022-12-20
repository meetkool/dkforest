-- +migrate Up
ALTER TABLE users ADD COLUMN login_attempts INTEGER NOT NULL DEFAULT 0;

-- +migrate Down
