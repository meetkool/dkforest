-- +migrate Up
ALTER TABLE users ADD COLUMN poker_xmr_sub_address VARCHAR(255) NOT NULL DEFAULT '';
CREATE INDEX users_poker_xmr_sub_address_idx ON users (poker_xmr_sub_address);

CREATE TABLE IF NOT EXISTS poker_xmr_transactions (
    id INTEGER NOT NULL PRIMARY KEY,
    tx_id VARCHAR(255) NOT NULL,
    user_id INTEGER NOT NULL,
    address VARCHAR(255) NOT NULL,
    amount INTEGER NOT NULL,
    height INTEGER NOT NULL,
    confirmations INTEGER NOT NULL,
    is_in TINYINT(1) NOT NULL DEFAULT 0,
    processed TINYINT(1) NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT poker_xmr_transactions_user_id_fk
        FOREIGN KEY (user_id)
            REFERENCES users (id)
            ON DELETE NO ACTION
            ON UPDATE CASCADE);
CREATE INDEX poker_xmr_transactions_user_id_idx ON poker_xmr_transactions (user_id);
CREATE INDEX poker_xmr_transactions_is_in_idx ON poker_xmr_transactions (is_in);
CREATE INDEX poker_xmr_transactions_tx_id_idx ON poker_xmr_transactions (tx_id);

-- +migrate Down
