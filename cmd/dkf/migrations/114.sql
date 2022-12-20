-- +migrate Up
ALTER TABLE users ADD COLUMN gpg_two_factor_mode TINYINT(1) DEFAULT 0;

-- +migrate Down
