-- +migrate Up
ALTER TABLE settings ADD COLUMN maybe_auth_enabled TINYINT(1) NOT NULL DEFAULT 1;

-- +migrate Down
