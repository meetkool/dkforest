-- +migrate Up
ALTER TABLE filedrops ADD COLUMN password BLOB NULL;

-- +migrate Down
