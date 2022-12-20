-- +migrate Up
ALTER TABLE users ADD COLUMN secret_phrase BLOB NULL;

-- +migrate Down
