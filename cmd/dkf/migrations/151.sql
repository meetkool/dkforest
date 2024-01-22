-- +migrate Up
ALTER TABLE settings ADD COLUMN poker_withdraw_enabled TINYINT(1) NOT NULL DEFAULT 1;

-- +migrate Down
