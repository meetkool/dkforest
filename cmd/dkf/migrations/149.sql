-- +migrate Up
ALTER TABLE users ADD COLUMN xmr_balance INTEGER NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN xmr_balance_stagenet INTEGER NOT NULL DEFAULT 0;

-- +migrate Down
