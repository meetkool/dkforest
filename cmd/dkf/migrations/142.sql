-- +migrate Up
CREATE TABLE IF NOT EXISTS chess_games (
    id INTEGER NOT NULL PRIMARY KEY,
    uuid VARCHAR(100) UNIQUE NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NULL DEFAULT CURRENT_TIMESTAMP,
    white_user_id INTEGER NOT NULL,
    black_user_id INTEGER NOT NULL,
    pgn TEXT NOT NULL,
    outcome VARCHAR(20) NOT NULL DEFAULT '*',
    accuracy_white REAL NOT NULL DEFAULT 0,
    accuracy_black REAL NOT NULL DEFAULT 0,
    CONSTRAINT chess_games_white_user_id_fk
        FOREIGN KEY (white_user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE,
    CONSTRAINT chess_games_black_user_id_fk
        FOREIGN KEY (black_user_id)
            REFERENCES users (id)
            ON DELETE CASCADE
            ON UPDATE CASCADE);

-- +migrate Down
