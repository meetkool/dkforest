-- +migrate Up
ALTER TABLE users ADD COLUMN hide_ignored_users_from_list TINYINT(1) NOT NULL DEFAULT 0;

-- +migrate Down
