-- +migrate Up
ALTER TABLE users ADD COLUMN manual_multiline TINYINT(1) NOT NULL DEFAULT 0;

-- +migrate Down
