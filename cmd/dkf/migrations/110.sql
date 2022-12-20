-- +migrate Up
ALTER TABLE users ADD COLUMN autocomplete_commands_enabled TINYINT(1) NOT NULL DEFAULT 0;

-- +migrate Down
