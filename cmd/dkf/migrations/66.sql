-- +migrate Up
CREATE INDEX chat_messages_moderators_idx ON chat_messages (moderators);

-- +migrate Down
