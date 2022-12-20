-- +migrate Up
ALTER TABLE chat_inbox_messages ADD COLUMN moderators TINYINT(1) NOT NULL DEFAULT 0;

-- +migrate Down
