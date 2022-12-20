-- +migrate Up
ALTER TABLE chat_messages ADD COLUMN is_hellbanned TINYINT(1) NOT NULL DEFAULT 0;
CREATE INDEX chat_messages_is_hellbanned_idx ON chat_messages (is_hellbanned);

-- +migrate Down
