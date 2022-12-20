-- +migrate Up
ALTER TABLE users ADD COLUMN display_ignored TINYINT(1) NOT NULL DEFAULT 0;

-- +migrate Down
