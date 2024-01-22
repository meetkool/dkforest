-- +migrate Up
ALTER TABLE poker_xmr_transactions ADD COLUMN status INTEGER NOT NULL DEFAULT 0;
CREATE INDEX poker_xmr_transactions_status_idx ON poker_xmr_transactions(status);
UPDATE poker_xmr_transactions SET status = 2 WHERE is_in = 0 AND status = 0;

-- +migrate Down
