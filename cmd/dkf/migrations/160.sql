-- +migrate Up
ALTER TABLE users ADD COLUMN use_stream_menu TINYINT(1) NOT NULL DEFAULT 0;

-- +migrate Down
