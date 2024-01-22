-- +migrate Up
ALTER TABLE users ADD COLUMN code_block_height INTEGER DEFAULT 300;

-- +migrate Down
