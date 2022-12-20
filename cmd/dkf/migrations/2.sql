-- +migrate Up
CREATE TABLE IF NOT EXISTS onion_blacklists (
    md5 VARCHAR(32) NOT NULL PRIMARY KEY,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP);

-- +migrate Down
