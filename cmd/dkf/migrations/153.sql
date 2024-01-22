-- +migrate Up
ALTER TABLE poker_xmr_transactions ADD COLUMN fee INTEGER NOT NULL DEFAULT 0;

-- +migrate Down
