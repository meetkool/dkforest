-- +migrate Up
ALTER TABLE users ADD COLUMN karma INTEGER NOT NULL DEFAULT 0;

-- +migrate Down
