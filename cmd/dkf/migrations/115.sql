-- +migrate Up
ALTER TABLE settings ADD COLUMN silent_self_kick TINYINT(1) NOT NULL DEFAULT 1;

-- +migrate Down
