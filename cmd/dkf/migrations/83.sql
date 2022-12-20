-- +migrate Up
CREATE INDEX downloads_user_id_idx ON downloads (user_id);
CREATE INDEX downloads_filename_idx ON downloads (filename);

-- +migrate Down
