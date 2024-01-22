-- +migrate Up
ALTER TABLE users ADD COLUMN can_use_uppercase TINYINT(1) NOT NULL DEFAULT 1;

-- +migrate Down
