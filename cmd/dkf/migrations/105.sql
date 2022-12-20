-- +migrate Up
ALTER TABLE users ADD COLUMN afk TINYINT(1) NOT NULL DEFAULT 0;

-- +migrate Down
