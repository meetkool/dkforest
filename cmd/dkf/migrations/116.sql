-- +migrate Up
ALTER TABLE users ADD COLUMN spellcheck_enabled TINYINT(1) NOT NULL DEFAULT 0;

-- +migrate Down
