-- +migrate Up
CREATE INDEX users_poker_referred_by_idx ON users (poker_referred_by);

-- +migrate Down
