-- +migrate Up
ALTER TABLE users ADD COLUMN captcha_required TINYINT(1) NOT NULL DEFAULT 0;

-- +migrate Down
