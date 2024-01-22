-- +migrate Up
ALTER TABLE chat_rooms ADD COLUMN read_only TINYINT(1) NOT NULL DEFAULT 0;

-- +migrate Down
