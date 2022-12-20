-- +migrate Up
ALTER TABLE users ADD COLUMN block_new_users_pm TINYINT(1) NOT NULL DEFAULT 0;

-- +migrate Down
