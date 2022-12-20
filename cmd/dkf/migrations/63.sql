-- +migrate Up
ALTER TABLE users ADD COLUMN age_public_key VARCHAR(255) NOT NULL DEFAULT '';

-- +migrate Down
