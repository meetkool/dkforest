-- +migrate Up
ALTER TABLE users ADD COLUMN collect_metadata TINYINT(1) NOT NULL DEFAULT 0;

-- +migrate Down
