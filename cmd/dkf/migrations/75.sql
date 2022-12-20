-- +migrate Up
ALTER TABLE users ADD COLUMN signup_metadata TEXT NOT NULL DEFAULT '';

-- +migrate Down
