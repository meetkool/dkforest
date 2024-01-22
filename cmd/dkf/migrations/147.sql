-- +migrate Up
CREATE TABLE IF NOT EXISTS poker_tables (
    id INTEGER NOT NULL PRIMARY KEY,
    slug VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(50) NOT NULL,
    min_buy_in INTEGER NOT NULL,
    max_buy_in INTEGER NOT NULL,
    min_bet INTEGER NOT NULL,
    is_test TINYINT(1) NOT NULL DEFAULT 1);
CREATE INDEX poker_tables_slug_idx ON poker_tables (slug);
CREATE INDEX poker_tables_is_test_idx ON poker_tables (is_test);

INSERT INTO poker_tables (slug, name, min_buy_in, max_buy_in, min_bet, is_test) VALUES ('test', 'test', 1000, 2000, 20, 1);
INSERT INTO poker_tables (slug, name, min_buy_in, max_buy_in, min_bet, is_test) VALUES ('test1', 'test1', 1000, 2000, 20, 1);
INSERT INTO poker_tables (slug, name, min_buy_in, max_buy_in, min_bet, is_test) VALUES ('test2', 'test2', 1000, 2000, 20, 1);
INSERT INTO poker_tables (slug, name, min_buy_in, max_buy_in, min_bet, is_test) VALUES ('test3', 'test3', 1000, 2000, 20, 1);
INSERT INTO poker_tables (slug, name, min_buy_in, max_buy_in, min_bet, is_test) VALUES ('test4', 'test4', 1000, 2000, 20, 1);
INSERT INTO poker_tables (slug, name, min_buy_in, max_buy_in, min_bet, is_test) VALUES ('test5', 'test5', 1000, 2000, 20, 1);

CREATE TABLE IF NOT EXISTS poker_table_accounts (
    id INTEGER NOT NULL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    poker_table_id INTEGER NOT NULL,
    amount INTEGER NOT NULL,
    amount_bet INTEGER NOT NULL,
    UNIQUE (user_id, poker_table_id),
    CONSTRAINT poker_table_accounts_user_id_fk
        FOREIGN KEY (user_id)
            REFERENCES users (id)
            ON DELETE NO ACTION
            ON UPDATE CASCADE,
    CONSTRAINT poker_table_accounts_poker_table_id_fk
        FOREIGN KEY (poker_table_id)
            REFERENCES poker_tables (id)
            ON DELETE NO ACTION
            ON UPDATE CASCADE);
CREATE INDEX poker_table_accounts_poker_table_id_idx ON poker_table_accounts (poker_table_id);
CREATE INDEX poker_table_accounts_user_id_idx ON poker_table_accounts (user_id);
CREATE INDEX poker_table_accounts_amount_idx ON poker_table_accounts (amount);
CREATE INDEX poker_table_accounts_amount_bet_idx ON poker_table_accounts (amount_bet);

-- +migrate Down
