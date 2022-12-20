-- +migrate Up
ALTER TABLE users ADD COLUMN can_see_hellbanned TINYINT(1) NOT NULL DEFAULT 0;

-- +migrate Down
