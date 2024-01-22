-- +migrate Up
ALTER TABLE chat_messages ADD COLUMN rev INTEGER NOT NULL DEFAULT 0;

-- +migrate Down
