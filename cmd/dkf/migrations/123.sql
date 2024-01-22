-- +migrate Up
ALTER TABLE filedrops ADD COLUMN iv BLOB NULL;

-- +migrate Down
