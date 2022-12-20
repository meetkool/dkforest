-- +migrate Up
ALTER TABLE settings ADD COLUMN force_login_captcha TINYINT(1) NOT NULL DEFAULT 0;

-- +migrate Down
