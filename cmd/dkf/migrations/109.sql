-- +migrate Up
ALTER TABLE users ADD COLUMN notify_chess_games TINYINT(1) NOT NULL DEFAULT 0;
CREATE INDEX users_notify_chess_games_idx ON users (notify_chess_games);

-- +migrate Down
