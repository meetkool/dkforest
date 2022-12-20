-- +migrate Up
ALTER TABLE users ADD COLUMN chat_read_marker_color VARCHAR(50) NOT NULL DEFAULT '#4e7597';
ALTER TABLE users ADD COLUMN chat_read_marker_size INTEGER NOT NULL DEFAULT 1;

-- +migrate Down
