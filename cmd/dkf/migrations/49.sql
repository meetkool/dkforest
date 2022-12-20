-- +migrate Up
ALTER TABLE users ADD COLUMN display_kick_button TINYINT(1) NOT NULL DEFAULT 0;

-- +migrate Down
