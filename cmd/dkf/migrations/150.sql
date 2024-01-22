-- +migrate Up
CREATE TABLE IF NOT EXISTS poker_casino (
    lock char(1) not null DEFAULT 'X',
    id INTEGER NOT NULL DEFAULT 1,
    rake INTEGER NOT NULL DEFAULT 0,
    constraint pk_poker_casino_RestrictToOneRow PRIMARY KEY (lock),
    constraint CK_poker_casino_Locked CHECK (lock='X'));

-- +migrate Down
