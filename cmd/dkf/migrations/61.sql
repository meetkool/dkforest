-- +migrate Up
ALTER TABLE users ADD COLUMN terminate_all_sessions_on_logout TINYINT(1) NOT NULL DEFAULT 1;

-- +migrate Down
