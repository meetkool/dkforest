-- +migrate Up
ALTER TABLE users ADD COLUMN chips_test INTEGER NOT NULL DEFAULT 0;

-- +migrate Down
