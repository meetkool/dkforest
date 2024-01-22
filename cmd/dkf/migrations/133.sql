-- +migrate Up
ALTER TABLE settings ADD COLUMN pow_enabled TINYINT(1) NOT NULL DEFAULT 0;

-- +migrate Down
