-- +migrate Up
ALTER TABLE users ADD COLUMN use_stream TINYINT(1) NOT NULL DEFAULT 0;

-- +migrate Down
