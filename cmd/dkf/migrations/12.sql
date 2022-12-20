-- +migrate Up
CREATE TABLE IF NOT EXISTS settings (
    id INTEGER NOT NULL PRIMARY KEY,
    signup_enabled TINYINT(1) NOT NULL DEFAULT 1);

-- +migrate Down
