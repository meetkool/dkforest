-- +migrate Up
ALTER TABLE forum_messages ADD COLUMN is_signed TINYINT(1) NOT NULL DEFAULT 0;

-- +migrate Down
