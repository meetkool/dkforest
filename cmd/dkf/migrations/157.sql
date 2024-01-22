-- +migrate Up
ALTER TABLE users ADD COLUMN poker_referred_by INTEGER NULL;
ALTER TABLE users ADD COLUMN poker_referral_token VARCHAR(50) NULL;
ALTER TABLE users ADD COLUMN poker_rake_back INTEGER NOT NULL DEFAULT 0;

CREATE UNIQUE INDEX users_poker_referral_token_uniq ON users (poker_referral_token);

-- +migrate Down
