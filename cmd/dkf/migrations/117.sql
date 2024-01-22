-- +migrate Up
CREATE TABLE IF NOT EXISTS prohibited_passwords (
    password VARCHAR(50) NOT NULL PRIMARY KEY
);

-- +migrate Down
