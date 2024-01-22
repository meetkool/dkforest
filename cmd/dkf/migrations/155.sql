-- +migrate Up
ALTER TABLE settings ADD COLUMN monero_price REAL NOT NULL DEFAULT 170;

-- +migrate Down
