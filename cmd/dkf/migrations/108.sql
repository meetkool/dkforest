-- +migrate Up
ALTER TABLE users ADD COLUMN chat_bar_at_bottom TINYINT(1) NOT NULL DEFAULT 0;

-- +migrate Down
