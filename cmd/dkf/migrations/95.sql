-- +migrate Up
ALTER TABLE users ADD COLUMN is_incognito TINYINT(1) NOT NULL DEFAULT 0;

-- +migrate Down
