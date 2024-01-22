-- +migrate Up
ALTER TABLE poker_tables ADD COLUMN idx INTEGER NOT NULL DEFAULT 0;

-- +migrate Down
