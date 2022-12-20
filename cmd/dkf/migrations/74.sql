-- +migrate Up
ALTER TABLE users ADD COLUMN can_use_multiline TINYINT(1) NOT NULL DEFAULT 0;

-- +migrate Down
