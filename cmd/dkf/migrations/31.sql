-- +migrate Up
ALTER TABLE chat_rooms ADD COLUMN is_ephemeral TINYINT(1) DEFAULT 1;

CREATE INDEX rooms_is_ephemeral_idx ON chat_rooms (is_ephemeral);

-- +migrate Down
