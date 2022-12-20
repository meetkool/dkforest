-- +migrate Up
ALTER TABLE users ADD COLUMN duress_password VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN is_under_duress TINYINT(1) NOT NULL DEFAULT 0;

-- +migrate Down
