-- +migrate Up
ALTER TABLE chat_rooms ADD COLUMN external_link VARCHAR(255) NOT NULL DEFAULT '';

-- +migrate Down
