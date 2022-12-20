-- +migrate Up
ALTER TABLE settings ADD COLUMN captcha_difficulty INTEGER NOT NULL DEFAULT 1;

-- +migrate Down
