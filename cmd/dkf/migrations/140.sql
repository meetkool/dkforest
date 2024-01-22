-- +migrate Up
ALTER TABLE users ADD COLUMN confirm_external_links TINYINT(1) NOT NULL DEFAULT 0;

-- +migrate Down
