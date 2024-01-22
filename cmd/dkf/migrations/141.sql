-- +migrate Up
ALTER TABLE users ADD COLUMN chess_sounds_enabled TINYINT(1) NOT NULL DEFAULT 0;

-- +migrate Down
