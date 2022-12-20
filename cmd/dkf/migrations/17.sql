-- +migrate Up
ALTER TABLE users ADD COLUMN avatar BLOB NULL;

-- +migrate Down
