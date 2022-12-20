-- +migrate Up
ALTER TABLE users ADD COLUMN theme INTEGER NOT NULL DEFAULT 0;

-- +migrate Down
