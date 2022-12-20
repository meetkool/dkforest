-- +migrate Up
ALTER TABLE users ADD COLUMN email VARCHAR(255) NULL;
ALTER TABLE users ADD COLUMN website VARCHAR(255) NULL;

-- +migrate Down
