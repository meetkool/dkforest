-- +migrate Up
ALTER TABLE settings ADD COLUMN home_users_list TINYINT(1) NOT NULL DEFAULT 0;

-- +migrate Down
