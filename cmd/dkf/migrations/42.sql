-- +migrate Up
ALTER TABLE users ADD COLUMN display_pms INTEGER NOT NULL DEFAULT 0;

-- +migrate Down
