-- +migrate Up

CREATE TABLE IF NOT EXISTS filedrops (
    id INTEGER NOT NULL PRIMARY KEY,
    uuid VARCHAR(100) UNIQUE NOT NULL,
    file_name VARCHAR(255) UNIQUE NOT NULL,
    orig_file_name VARCHAR(255) NOT NULL,
    file_size INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NULL DEFAULT CURRENT_TIMESTAMP);

-- +migrate Down
