-- +migrate Up
ALTER TABLE users ADD COLUMN poker_sounds_enabled TINYINT(1) NOT NULL DEFAULT 0;

-- +migrate Down
