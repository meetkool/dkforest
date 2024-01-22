-- +migrate Up
ALTER TABLE users ADD COLUMN hellban_opacity INTEGER DEFAULT 30;

-- +migrate Down
