-- +migrate Up
CREATE TABLE IF NOT EXISTS memes (
    id INTEGER PRIMARY KEY,
    slug VARCHAR(255) NOT NULL UNIQUE,
    file_name VARCHAR(255) UNIQUE NOT NULL,
    orig_file_name VARCHAR(255) NOT NULL,
    file_size INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);

-- +migrate Down
