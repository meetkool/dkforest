-- +migrate Up
ALTER TABLE settings ADD COLUMN downloads_enabled TINYINT(1) NOT NULL DEFAULT 1;

-- +migrate Down
