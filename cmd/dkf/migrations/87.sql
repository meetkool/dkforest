-- +migrate Up
ALTER TABLE users ADD COLUMN display_hellban_button TINYINT(1) NOT NULL DEFAULT 1;
ALTER TABLE users ADD COLUMN display_delete_button TINYINT(1) NOT NULL DEFAULT 1;

-- +migrate Down
