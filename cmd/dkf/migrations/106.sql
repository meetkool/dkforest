-- +migrate Up
ALTER TABLE users ADD COLUMN notify_chess_move TINYINT(1) NOT NULL DEFAULT 0;

-- +migrate Down
