-- +migrate Up
ALTER TABLE users ADD COLUMN chat_tutorial INTEGER NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN chat_tutorial_time DATETIME NULL;

-- +migrate Down
