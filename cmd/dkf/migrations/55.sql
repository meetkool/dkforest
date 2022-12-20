-- +migrate Up
ALTER TABLE chat_inbox_messages ADD COLUMN is_pm TINYINT(1) DEFAULT 0;

-- +migrate Down
