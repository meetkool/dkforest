-- +migrate Up
ALTER TABLE chat_rooms ADD COLUMN is_listed TINYINT(1) DEFAULT 0;

CREATE INDEX rooms_is_listed_idx ON chat_rooms (is_listed);

-- +migrate Down
