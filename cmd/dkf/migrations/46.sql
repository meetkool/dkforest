-- +migrate Up
ALTER TABLE settings ADD COLUMN protect_home TINYINT(1) NOT NULL DEFAULT 0;

-- +migrate Down
