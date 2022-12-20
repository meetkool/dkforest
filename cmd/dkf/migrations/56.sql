-- +migrate Up
ALTER TABLE users ADD COLUMN registration_duration INTEGER DEFAULT 0;

-- +migrate Down
