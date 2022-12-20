-- +migrate Up
CREATE INDEX chat_messages_created_at_idx ON chat_messages (created_at);
ALTER TABLE users ADD COLUMN chat_font INTEGER DEFAULT 1 NOT NULL;
ALTER TABLE users ADD COLUMN chat_bold TINYINT DEFAULT 0 NOT NULL;
ALTER TABLE users ADD COLUMN chat_italic TINYINT DEFAULT 0 NOT NULL;

-- +migrate Down
