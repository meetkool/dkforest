-- +migrate Up
ALTER TABLE settings ADD COLUMN signup_fake_enabled TINYINT(1) NOT NULL DEFAULT 0;

-- +migrate Down
