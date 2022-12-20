-- +migrate Up
ALTER TABLE users ADD COLUMN hide_right_column TINYINT(1) NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN date_format INTEGER NOT NULL DEFAULT 0;

-- +migrate Down
