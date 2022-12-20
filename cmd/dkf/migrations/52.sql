-- +migrate Up
ALTER TABLE users ADD COLUMN display_moderators TINYINT(1) NOT NULL DEFAULT 1;

-- +migrate Down
