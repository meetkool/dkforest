-- +migrate Up
ALTER TABLE poker_casino ADD COLUMN total_rake_back INTEGER NOT NULL DEFAULT 0;

-- +migrate Down
