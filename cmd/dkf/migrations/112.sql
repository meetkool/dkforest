-- +migrate Up
ALTER TABLE users ADD COLUMN afk_indicator_enabled TINYINT(1) NOT NULL DEFAULT 0;

-- +migrate Down
