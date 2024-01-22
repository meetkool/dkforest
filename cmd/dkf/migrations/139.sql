-- +migrate Up
ALTER TABLE users ADD COLUMN syntax_highlight_code VARCHAR(20) NOT NULL DEFAULT '';

-- +migrate Down
