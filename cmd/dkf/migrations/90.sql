-- +migrate Up
ALTER TABLE users ADD COLUMN can_change_color TINYINT(1) NOT NULL DEFAULT 1;

-- +migrate Down
