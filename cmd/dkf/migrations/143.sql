-- +migrate Up
ALTER TABLE chess_games ADD COLUMN stats BLOB NULL;

-- +migrate Down
