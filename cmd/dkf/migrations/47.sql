-- +migrate Up
ALTER TABLE users ADD COLUMN can_use_forum TINYINT(1) DEFAULT 1;

-- +migrate Down
