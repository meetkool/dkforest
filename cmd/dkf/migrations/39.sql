-- +migrate Up
ALTER TABLE users ADD COLUMN can_upload_file TINYINT(1) DEFAULT 1;

-- +migrate Down
