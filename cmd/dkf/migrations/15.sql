-- +migrate Up
ALTER TABLE users ADD COLUMN temp TINYINT(1) NOT NULL DEFAULT 0;

-- +migrate Down
