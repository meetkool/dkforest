-- +migrate Up
ALTER TABLE users ADD COLUMN chat_background_color VARCHAR(20) DEFAULT '#111111' NOT NULL;

-- +migrate Down
